package logicmonitor

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/grafana/grafana-logicmonitor-datasource-backend/pkg/cache"
	"github.com/grafana/grafana-logicmonitor-datasource-backend/pkg/constants"
	httpclient "github.com/grafana/grafana-logicmonitor-datasource-backend/pkg/httpclient"
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
func OldAndNewDataProcessorTask(query backend.DataQuery, queryModel models.QueryModel, metaData models.MetaData, authSettings *models.AuthSettings,
	pluginSettings *models.PluginSettings, mainTaskWaitGroup *sync.WaitGroup, logger log.Logger) backend.DataResponse {

	response := backend.DataResponse{}
	rawDataMap := make(map[int]*models.MultiInstanceRawData)

	/*
		Step 2
		if from queryEditor then get new data and append it,
		Note : same number of old entries are not removed as this is a temprary data used while on queryEditor making changes
	*/
	waitSec := GetWaitTimeInSec(metaData, queryModel.CollectInterval)
	if metaData.IsCallFromQueryEditor {
		if cacheData, present := cache.GetQueryEditorCacheData(metaData.TempQueryEditorID); present {
			rawDataMap = cacheData.(map[int]*models.MultiInstanceRawData)
		}
	} else {
		if frameValue, framePresent := cache.GetData(metaData.FrameId); framePresent {
			response = frameValue.(backend.DataResponse)
		}
	}

	lastRawDataEntryTimestamp := cache.GetLastestRawDataEntryTimestamp(metaData)
	if metaData.IsForLastXTime && waitSec == 0 {
		/*
			Step 1
			1. Get time range for rate limits records, multiple call will be made to each time range
		*/
		var timeRangeForApiCall []models.PendingTimeRange
		if lastRawDataEntryTimestamp == 0 {
			lastRawDataEntryTimestamp = UnixTruncateToNearestMinute(query.TimeRange.From.Unix(), 60)
		} else {
			lastRawDataEntryTimestamp++ // LastTimeStamp, increase by 1 second
		}
		if constants.EnableFetchDataTimeRange {
			timeRangeForApiCall = GetTimeRanges(lastRawDataEntryTimestamp, query.TimeRange.To.Unix(), queryModel.CollectInterval, metaData, logger)
		} else {
			timeRangeForApiCall = append(timeRangeForApiCall, models.PendingTimeRange{From: lastRawDataEntryTimestamp, To: query.TimeRange.To.Unix()})
		}
		if len(timeRangeForApiCall) == 0 {
			logger.Warn("Got no TimeRange for API call, try again, it should work!")
		} else if len(timeRangeForApiCall) > constants.NumberOfRecordsWithRateLimit {
			response.Error = fmt.Errorf(constants.APICallSMoreThanRateLimit, len(timeRangeForApiCall))
		}
		logger.Info("Number of API  Calls for this query are => ", len(timeRangeForApiCall))

		/*
			Step 3
			1. Initiate goroutines to call API for each time range caclulated
		*/
		wg := &sync.WaitGroup{}
		dataLenIdx := len(rawDataMap)
		appendRequest := false
		if dataLenIdx > 0 {
			appendRequest = true
		}
		for i := 0; i < len(timeRangeForApiCall) && response.Error == nil; i++ {
			wg.Add(1)
			go func(wg *sync.WaitGroup, rawDataLen int, from int64, to int64, metaData models.MetaData) backend.DataResponse {
				defer wg.Done()
				// Data is not present in the cache so fresh data needs to be fetched
				rawData, err := callRawDataAPI(&queryModel, pluginSettings, authSettings, from, to, metaData, logger)
				if err != nil && response.Error == nil {
					response.Error = err
				} else {
					rawData.Data.AppendRequest = appendRequest
					rawDataMap[rawDataLen] = &rawData
				}
				return response
			}(wg, dataLenIdx, timeRangeForApiCall[i].From, timeRangeForApiCall[i].To, metaData)
			dataLenIdx++
		}
		wg.Wait()

		// This block is only to log maxNewRawDataCountForAnyInstance when API is called, for debug purpose, can be removed
		if len(rawDataMap) > 0 {
			for i := 0; i < len(rawDataMap); i++ {
				var minNumOfEntriesFromInstances int = 0
				for _, newTimeValue := range rawDataMap[i].Data.Instances {
					if minNumOfEntriesFromInstances < len(newTimeValue.Time) {
						minNumOfEntriesFromInstances = len(newTimeValue.Time)
					}
				}
				logger.Info("Number of entries from API call =>", i, minNumOfEntriesFromInstances)
			}
		}
	} else {
		if metaData.IsCallFromQueryEditor {
			logger.Info("From QueryEditorCache : Remaining seconds for new API call => ", waitSec)
		} else {
			logger.Info("From FrameCache : Remaining seconds for new API call => ", waitSec)
		}
	}
	/*
		Step 4
		1. Build frame from results of all API calls. Append results from subsequent API calls in accordance to time ranges.
		2. Get lastest timestamp of radwdata from instances
	*/
	var dataFrameMap = make(map[string]*data.Frame)
	if response.Error == nil {
		for i := 0; i < len(rawDataMap); i++ {
			dataFrameMap, metaData.MatchedInstances = BuildFrameFromMultiInstance(metaData.FrameId, &queryModel, &rawDataMap[i].Data, dataFrameMap, metaData, logger)
		}
	}
	/*
		Step 5
		1. Check for Errors if any else store results in cache and return
	*/
	if len(rawDataMap) > 0 && !metaData.MatchedInstances && len(dataFrameMap) == 0 {
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
	logger.Info("LastRawDataEntryTimestamp => ", time.UnixMilli(lastRawDataEntryTimestamp*1000))
	mainTaskWaitGroup.Done()
	return response
}

// Gets fresh data by calling rest API
func callRawDataAPI(queryModel *models.QueryModel, pluginSettings *models.PluginSettings,
	authSettings *models.AuthSettings, from int64, to int64, metaData models.MetaData, logger log.Logger) (models.MultiInstanceRawData, error) {
	var rawData models.MultiInstanceRawData //nolint:exhaustivestruct

	fullPath := BuildURLReplacingQueryParams(constants.RawDataMultiInstanceReq, queryModel, from, to, metaData)

	logger.Debug("Calling API  => ", pluginSettings.Path, fullPath)
	//todo remove the loggers

	respByte, err := httpclient.Get(pluginSettings, authSettings, fullPath, logger)
	if err != nil {
		logger.Error("Error from server => ", err)

		return rawData, err //nolint:wrapcheck
	}

	err = json.Unmarshal(respByte, &rawData)
	if err != nil {
		logger.Error(constants.ErrorUnmarshallingErrorData+"raw-data => ", err)

		return rawData, err //nolint:wrapcheck
	}

	if rawData.Error != "OK" {
		return rawData, errors.New(rawData.Error)
	}

	return rawData, nil
}
