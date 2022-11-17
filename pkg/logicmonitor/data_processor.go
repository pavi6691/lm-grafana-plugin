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

	/*
		Step 2
		Get Data from respectvie Caches, data from each cache has differrent processing machanism
	*/
	// if metaData.IsCallFromQueryEditor {
	// 	if cacheData, present := cache.GetData(metaData.TempQueryEditorID); present {
	// 		rawDataMap = cacheData.(map[int]*models.MultiInstanceRawData)
	// 	}
	// } else {
	// 	if cacheData, present := cache.GetData(metaData.FrameId); present {
	// 		rawDataMap = cacheData.(map[int]*models.MultiInstanceRawData)
	// 	}
	// }

	/*
		Step 1
		1. wait time is over/requets then calculate time range. Expect a new data if timeRangeForApiCall has entry
		2. Caclulate time range for rate limits records, multiple call will be made to each time range
	*/
	var prependTimeRangeForApiCall []models.PendingTimeRange
	var appendTimeRangeForApiCall []models.PendingTimeRange
	firstRawDataEntryTimestamp := cache.GetFirstRawDataEntryTimestamp(metaData, queryModel.EnableDataAppendFeature)
	lastRawDataEntryTimestamp := cache.GetLastestRawDataEntryTimestamp(metaData, queryModel.EnableDataAppendFeature)
	needToPrependData := UnixTruncateToNearestMinute(query.TimeRange.From.Unix(), 60) < UnixTruncateToNearestMinute(firstRawDataEntryTimestamp, 60)
	if needToPrependData {
		prependTimeRangeForApiCall = GetTimeRanges(UnixTruncateToNearestMinute(query.TimeRange.From.Unix(), 60), firstRawDataEntryTimestamp-1, queryModel.CollectInterval, metaData, logger)
	}
	waitSec := GetWaitTimeInSec(metaData, queryModel.CollectInterval, queryModel.EnableDataAppendFeature)
	needToAppendData := waitSec == 0
	logger.Info("firstRawDataEntryTimestamp", time.UnixMilli(firstRawDataEntryTimestamp*1000))
	logger.Info("lastRawDataEntryTimestamp", time.UnixMilli(lastRawDataEntryTimestamp*1000))
	if (needToAppendData || needToPrependData || response.Error != nil) && (queryModel.EnableDataAppendFeature) {
		if lastRawDataEntryTimestamp > 0 {
			lastRawDataEntryTimestamp++ // LastTimeStamp, increase by 1 second
		} else {
			lastRawDataEntryTimestamp = UnixTruncateToNearestMinute(query.TimeRange.From.Unix(), 60)
		}
		if queryModel.EnableHistoricalData {
			appendTimeRangeForApiCall = GetTimeRanges(lastRawDataEntryTimestamp, query.TimeRange.To.Unix(), queryModel.CollectInterval, metaData, logger)

		} else {
			appendTimeRangeForApiCall = append(appendTimeRangeForApiCall, models.PendingTimeRange{From: lastRawDataEntryTimestamp, To: query.TimeRange.To.Unix()})
		}
		if len(prependTimeRangeForApiCall) == 0 && len(appendTimeRangeForApiCall) == 0 {
			logger.Warn("Got no TimeRange for API call, try again, it should work!")
		} else {
			apisCallsSofar := cache.GetApiCalls(pluginContext.DataSourceInstanceSettings.UID).NrOfCalls
			totalApis := apisCallsSofar + len(prependTimeRangeForApiCall) + len(appendTimeRangeForApiCall)
			allowedNrOfCalls := constants.NumberOfRecordsWithRateLimit - apisCallsSofar
			if totalApis > constants.NumberOfRecordsWithRateLimit {
				logger.Error(fmt.Sprintf(constants.RateLimitAuditMsg, apisCallsSofar, len(prependTimeRangeForApiCall)+len(appendTimeRangeForApiCall), totalApis, allowedNrOfCalls))
				response.Error = fmt.Errorf(constants.RateLimitAuditMsg, apisCallsSofar, len(prependTimeRangeForApiCall)+len(appendTimeRangeForApiCall), totalApis, allowedNrOfCalls)
			} else {
				cache.AddApiCalls(pluginContext.DataSourceInstanceSettings.UID, totalApis)
			}
			logger.Info(fmt.Sprintf(constants.RateLimitValidation, apisCallsSofar, len(prependTimeRangeForApiCall)+len(appendTimeRangeForApiCall), totalApis))
		}
	} else if queryModel.EnableDataAppendFeature {
		logger.Info(fmt.Sprintf("ID = %s Waiting for %d seconds for next data..", metaData.FrameId, waitSec))
	}

	//TODO remove start
	logger.Info("")
	//TODO remove end
	rawDataMap := make(map[int]*models.MultiInstanceRawData)
	dataMapLen := len(rawDataMap)
	dataFromApi := getDataFromApi(prependTimeRangeForApiCall, make(map[int]*models.MultiInstanceRawData), queryModel, metaData, authSettings, pluginSettings, logger)
	if len(dataFromApi) > 0 {
		//TODO remove start
		if len(dataFromApi) > 0 {
			tot := 0
			for i := 0; i < len(dataFromApi); i++ {
				var minNumOfEntriesFromInstances int = 0
				for _, newTimeValue := range dataFromApi[i].Data.Instances {
					if minNumOfEntriesFromInstances < len(newTimeValue.Time) {
						minNumOfEntriesFromInstances = len(newTimeValue.Time)
					}
				}
				tot = tot + minNumOfEntriesFromInstances
			}
			logger.Info("Prepend Number of entries", tot)
		}
		//TODO remove end
		for i := 0; i < len(dataFromApi); i++ {
			rawDataMap[dataMapLen] = dataFromApi[i]
			dataMapLen++
		}
	}
	if data, ok := cache.GetData(metaData.FrameId); ok {
		if cachedData, ok := data.(*models.MultiInstanceRawData); ok {
			rawDataMap[dataMapLen] = cachedData
			dataMapLen++
		}
		//TODO remove start
		var minNumOfEntriesFromInstances int = 0
		for _, newTimeValue := range rawDataMap[dataMapLen-1].Data.Instances {
			if minNumOfEntriesFromInstances < len(newTimeValue.Time) {
				minNumOfEntriesFromInstances = len(newTimeValue.Time)
			}
		}
		logger.Info("Cached Number of entries", minNumOfEntriesFromInstances)
		//TODO remove end
	}
	dataFromApi = getDataFromApi(appendTimeRangeForApiCall, make(map[int]*models.MultiInstanceRawData), queryModel, metaData, authSettings, pluginSettings, logger)
	if len(dataFromApi) > 0 {
		//TODO remove start
		if len(dataFromApi) > 0 {
			tot := 0
			for i := 0; i < len(dataFromApi); i++ {
				var minNumOfEntriesFromInstances int = 0
				for _, newTimeValue := range dataFromApi[i].Data.Instances {
					if minNumOfEntriesFromInstances < len(newTimeValue.Time) {
						minNumOfEntriesFromInstances = len(newTimeValue.Time)
					}
				}
				tot = tot + minNumOfEntriesFromInstances
			}
			logger.Info("Append Number of entries", tot)
		}
		//TODO remove end
		for i := 0; i < len(dataFromApi); i++ {
			rawDataMap[dataMapLen] = dataFromApi[i]
			dataMapLen++
		}
	}

	if len(rawDataMap) > 0 {
		tot := 0
		for i := 0; i < len(rawDataMap); i++ {
			var minNumOfEntriesFromInstances int = 0
			for _, newTimeValue := range rawDataMap[i].Data.Instances {
				if minNumOfEntriesFromInstances < len(newTimeValue.Time) {
					minNumOfEntriesFromInstances = len(newTimeValue.Time)
				}
			}
			tot = tot + minNumOfEntriesFromInstances
		}
		logger.Info("Total Number of entries", tot)
	}

	mergedfilteredData := cache.GetDataFor(metaData, query.TimeRange.From.Unix(), query.TimeRange.To.Unix(), rawDataMap, logger)
	if mergedfilteredData == nil {
		response.Error = errors.New(constants.NoDataFromLM)
	} else {
		// TODO remove start
		var minNumOfEntriesFromInstances int = 0
		for _, newTimeValue := range mergedfilteredData.Data.Instances {
			if minNumOfEntriesFromInstances < len(newTimeValue.Time) {
				minNumOfEntriesFromInstances = len(newTimeValue.Time)
			}
		}
		logger.Info("Filtered Number of entries", minNumOfEntriesFromInstances)
		//TODO remove end

	}
	//TODO remove start
	logger.Info("")
	//TODO remove end
	/*
		Step 4
		1. Build frame from results of all API calls. Append results from subsequent API calls in accordance to time ranges.
		2. Store lastest timestamp of radwdata from instances
	*/
	var dataFrameMap = make(map[string]*data.Frame)
	if response.Error == nil || response.Error.Error() == constants.RateLimitErrMsg {
		if mergedfilteredData.Error != "OK" {
			response.Error = errors.New(mergedfilteredData.Error)
		} else {
			dataFrameMap, metaData.MatchedInstances = BuildFrameFromMultiInstance(metaData.FrameId, &queryModel, &mergedfilteredData.Data, dataFrameMap, metaData, logger)
		}
	}
	/*
		Step 5
		1. Check for Errors if any else store results in cache and return
	*/
	if len(rawDataMap) > 0 && !metaData.MatchedInstances && len(dataFrameMap) == 0 && response.Error == nil {
		response.Error = errors.New(constants.InstancesNotMatchingWithHosts)
	} else if len(dataFrameMap) > 0 {
		response.Frames = nil
		for _, frame := range dataFrameMap {
			response.Frames = append(response.Frames, frame)
		}
		// Add data to cache
		if metaData.IsCallFromQueryEditor {
			cache.StoreData(metaData.TempQueryEditorID, (constants.QueryEditorCacheTTLInMinutes * 60), mergedfilteredData)
			// Store updated data in frame cache. this data returned when entry is deleted from editor temp cache.
			// this avoids new API calls for frame cache
			cache.StoreData(metaData.FrameId, metaData.FrameCacheTTLInSeconds, mergedfilteredData)
		} else {
			cache.StoreData(metaData.FrameId, metaData.FrameCacheTTLInSeconds, mergedfilteredData)
		}
	}
	return response
}

/*
1. Initiate goroutines to call API for each time range caclulated
*/
func getDataFromApi(timeRangeForApiCall []models.PendingTimeRange, rawDataMap map[int]*models.MultiInstanceRawData, queryModel models.QueryModel,
	metaData models.MetaData, authSettings *models.AuthSettings, pluginSettings *models.PluginSettings, logger log.Logger) map[int]*models.MultiInstanceRawData {
	if len(timeRangeForApiCall) > 0 {
		wg := &sync.WaitGroup{}
		dataLenIdx := len(rawDataMap)
		// nrOfConcurrentJobs := 5
		jobs := make(chan Job, len(timeRangeForApiCall))
		for i := 0; i < len(timeRangeForApiCall); i++ {
			wg.Add(1)
			go CallDataAPI(wg, jobs, rawDataMap, &queryModel, pluginSettings, authSettings, metaData, logger)
		}
		for i := 0; i < len(timeRangeForApiCall); i++ {
			jobs <- Job{JobId: dataLenIdx, TimeFrom: timeRangeForApiCall[i].From, TimeTo: timeRangeForApiCall[i].To}
			dataLenIdx++
		}
		close(jobs)
		wg.Wait()
	}
	return rawDataMap
}
