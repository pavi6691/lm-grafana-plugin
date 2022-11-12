package logicmonitor

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/grafana/grafana-logicmonitor-datasource-backend/pkg/cache"
	"github.com/grafana/grafana-logicmonitor-datasource-backend/pkg/constants"
	"github.com/grafana/grafana-logicmonitor-datasource-backend/pkg/models"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/grafana/grafana-plugin-sdk-go/data"
)

/*
Start Task. This task is executed

 1. Only once when query request is for fixed time range set. Only one time because results are not going to be changed for fixed time range.
    Results from cache will be returned in subsequent calls

 2. When query request is for last X time and datasource collect interval is over from last time when data is recieved

 3. When Query is updated and datasource collect interval is over from last time when data is recieved

    So very first time this task is executed. in subsequent calls it waits for datasource collect interval from last timestamp data recieved
*/
func GetData(query backend.DataQuery, queryModel models.QueryModel, metaData models.MetaData, authSettings *models.AuthSettings,
	pluginSettings *models.PluginSettings, pluginContext backend.PluginContext, logger log.Logger) backend.DataResponse {
	response := backend.DataResponse{}
	rawDataMap := make(map[int]*models.MultiInstanceRawData)

	/*
		Step 1
		1. Get data from framcache
	*/

	if frameValue, framePresent := cache.GetData(metaData.FrameId); framePresent {
		response = frameValue.(backend.DataResponse)
		if !queryModel.EnableDataAppendFeature {
			// backword compatible: if its not to append and entry is still present means its not yet 1 minute since data is fetched. so return entry from cache
			return response
		}
	}

	/*
		Step 1
		1. wait time is over/requets then calculate time range. Expect a new data if timeRangeForApiCall has entry
		2. Caclulate time range for rate limits records, multiple call will be made to each time range
	*/
	var timeRangeForApiCall []models.PendingTimeRange
	waitSec := GetWaitTimeInSec(metaData, queryModel.CollectInterval, queryModel.EnableDataAppendFeature)
	_, present := cache.GetQueryEditorCacheData(metaData.TempQueryEditorID)
	if (waitSec == 0 || response.Error != nil) && (queryModel.EnableDataAppendFeature || !present) {
		lastRawDataEntryTimestamp := cache.GetLastestRawDataEntryTimestamp(metaData, queryModel.EnableDataAppendFeature)
		if lastRawDataEntryTimestamp == 0 {
			lastRawDataEntryTimestamp = UnixTruncateToNearestMinute(query.TimeRange.From.Unix(), 60)
		} else {
			lastRawDataEntryTimestamp++ // LastTimeStamp, increase by 1 second
		}
		if queryModel.EnableHistoricalData {
			timeRangeForApiCall = GetTimeRanges(lastRawDataEntryTimestamp, query.TimeRange.To.Unix(), queryModel.CollectInterval, metaData, logger)
		} else {
			timeRangeForApiCall = append(timeRangeForApiCall, models.PendingTimeRange{From: lastRawDataEntryTimestamp, To: query.TimeRange.To.Unix()})
		}
		if len(timeRangeForApiCall) == 0 {
			logger.Warn("Got no TimeRange for API call, try again, it should work!")
		} else {
			apisCallsSofar := cache.GetApiCalls(pluginContext.DataSourceInstanceSettings.UID).NrOfCalls
			totalApis := apisCallsSofar + len(timeRangeForApiCall)
			allowedNrOfCalls := constants.NumberOfRecordsWithRateLimit - apisCallsSofar
			if totalApis > constants.NumberOfRecordsWithRateLimit {
				logger.Error(fmt.Sprintf(constants.RateLimitAuditMsg, apisCallsSofar, len(timeRangeForApiCall), totalApis, allowedNrOfCalls))
				// response.Error = fmt.Errorf(constants.RateLimitAuditMsg, apisCallsSofar, len(timeRangeForApiCall), totalApis, allowedNrOfCalls)
			} else {
				cache.AddApiCalls(pluginContext.DataSourceInstanceSettings.UID, totalApis)
			}
			logger.Info(fmt.Sprintf(constants.RateLimitValidation, apisCallsSofar, len(timeRangeForApiCall), totalApis))
		}
		logger.Info("timeRangeForApiCall =>", len(timeRangeForApiCall), time.UnixMilli(lastRawDataEntryTimestamp*1000))
	} else if queryModel.EnableDataAppendFeature {
		logger.Info(fmt.Sprintf("ID = %s Waiting for %d seconds for next data..", metaData.FrameId, waitSec))
	}

	/*
		Step 2
		Get Data from respectvie Caches, data from each cache has differrent processing machanism
	*/
	if metaData.IsCallFromQueryEditor {
		if cacheData, present := cache.GetQueryEditorCacheData(metaData.TempQueryEditorID); present {
			rawDataMap = cacheData.(map[int]*models.MultiInstanceRawData)
		}
	} else if len(timeRangeForApiCall) == 0 { // No new data, it from dashboard, return from cache
		return response
	}

	/*
		Step 3
		1. Initiate goroutines to call API for each time range caclulated
	*/
	metaData.AppendOnly = false
	if len(timeRangeForApiCall) > 0 {
		response.Error = nil
		wg := &sync.WaitGroup{}
		dataLenIdx := len(rawDataMap)
		AppendAndDelete := false
		if dataLenIdx > 0 {
			AppendAndDelete = true
		}
		// nrOfConcurrentJobs := 5
		jobs := make(chan Job, len(timeRangeForApiCall))
		for i := 0; i < len(timeRangeForApiCall); i++ {
			wg.Add(1)
			go CallDataAPI(wg, jobs, rawDataMap, &queryModel, pluginSettings, authSettings, metaData, AppendAndDelete, logger)
		}
		for i := 0; i < len(timeRangeForApiCall); i++ {
			jobs <- Job{JobId: dataLenIdx, TimeFrom: timeRangeForApiCall[i].From, TimeTo: timeRangeForApiCall[i].To}
			dataLenIdx++
		}
		close(jobs)
		wg.Wait()
	}

	// This block is only to log maxNewRawDataCountForAnyInstance when API is called, for debug purpose, can be removed
	// if len(rawDataMap) > 0 {
	// 	for i := 0; i < len(rawDataMap); i++ {
	// 		var minNumOfEntriesFromInstances int = 0
	// 		for _, newTimeValue := range rawDataMap[i].Data.Instances {
	// 			if minNumOfEntriesFromInstances < len(newTimeValue.Time) {
	// 				minNumOfEntriesFromInstances = len(newTimeValue.Time)
	// 			}
	// 		}
	// 		logger.Info("Number of entries from API call =>", i, minNumOfEntriesFromInstances)
	// 	}
	// }

	/*
		Step 4
		1. Build frame from results of all API calls. Append results from subsequent API calls in accordance to time ranges.
		2. Store lastest timestamp of radwdata from instances
	*/
	var dataFrameMap = make(map[string]*data.Frame)
	if response.Error == nil || response.Error.Error() == constants.RateLimitErrMsg {
		for i := 0; i < len(rawDataMap); i++ {
			if rawDataMap[i].Error != "OK" {
				response.Error = errors.New(rawDataMap[i].Error)
				break
			}
			metaData.AppendAndDelete = rawDataMap[i].AppendAndDelete
			dataFrameMap, metaData.MatchedInstances = BuildFrameFromMultiInstance(metaData.FrameId, &queryModel, &rawDataMap[i].Data, dataFrameMap, metaData, logger)
		}
	}
	/*
		Step 5
		1. Check for Errors if any else store results in cache and return
	*/
	logger.Debug("Size dataFrameMap is = ", len(dataFrameMap))
	if len(rawDataMap) > 0 && !metaData.MatchedInstances && len(dataFrameMap) == 0 && response.Error == nil {
		response.Error = errors.New(constants.InstancesNotMatchingWithHosts)
	} else if len(dataFrameMap) > 0 {
		response.Frames = nil
		for _, frame := range dataFrameMap {
			response.Frames = append(response.Frames, frame)
		}
		// Add data to cache
		if metaData.IsCallFromQueryEditor {
			cache.StoreQueryEditorTempData(metaData.TempQueryEditorID, (constants.QueryEditorCacheTTLInMinutes * 60), rawDataMap)
			// Store updated data in frame cache. this data returned when entry is deleted from editor temp cache.
			// this avoids new API calls for frame cache
			cache.StoreFrame(metaData.FrameId, metaData.FrameCacheTTLInSeconds, response)
		} else {
			cache.StoreFrame(metaData.FrameId, metaData.FrameCacheTTLInSeconds, response)
		}
	}
	return response
}
