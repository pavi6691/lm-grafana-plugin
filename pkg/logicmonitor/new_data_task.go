package logicmonitor

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend/log"

	"github.com/grafana/grafana-logicmonitor-datasource-backend/pkg/cache"
	"github.com/grafana/grafana-logicmonitor-datasource-backend/pkg/constants"
	httpclient "github.com/grafana/grafana-logicmonitor-datasource-backend/pkg/httpclient"
	"github.com/grafana/grafana-logicmonitor-datasource-backend/pkg/models"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
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
	/*
		Step 1
		1. Get time range for rate limits records, multiple call will be made to each time range
	*/
	var timeRangeForApiCall []models.PendingTimeRange
	var actualNumberOfCallsBeforeAppend int = 1
	if constants.EnableFetchDataTimeRange {
		timeRangeForApiCall = GetTimeRanges(query.TimeRange.From.Unix(), query.TimeRange.To.Unix(), queryModel.CollectInterval, metaData, logger)
		metaData.InstanceWithLastRawDataEntryTimestamp = 0
		actualNumberOfCallsBeforeAppend = len(GetTimeRanges(query.TimeRange.From.Unix(), query.TimeRange.To.Unix(), queryModel.CollectInterval, metaData, logger))
	} else {
		if metaData.InstanceWithLastRawDataEntryTimestamp == 0 {
			metaData.InstanceWithLastRawDataEntryTimestamp = query.TimeRange.From.Unix()
		}
		timeRangeForApiCall = append(timeRangeForApiCall, models.PendingTimeRange{From: metaData.InstanceWithLastRawDataEntryTimestamp, To: query.TimeRange.To.Unix()})
	}
	if len(timeRangeForApiCall) == 0 {
		logger.Warn("Got no TimeRange for API call, try again, it should work!")
		response, response.Error = GetFromFrameCache(metaData.FrameId)
		mainTaskWaitGroup.Done()
		return response
	} else if len(timeRangeForApiCall) > constants.NumberOfRecordsWithRateLimit {
		response.Error = fmt.Errorf(constants.APICallSMoreThanRateLimit, len(timeRangeForApiCall))
		mainTaskWaitGroup.Done()
		return response
	}
	logger.Info("Number of API  Calls for this query are => ", len(timeRangeForApiCall))
	rawDataMap := make(map[int]*models.MultiInstanceRawData)

	/*
		Step 2
		if from queryEditor then get new data and append it,
		Note : same number of old entries are not removed as this is a temprary data used while on queryEditor making changes
	*/
	if metaData.IsCallFromQueryEditor {
		cacheData, present := cache.GetQueryEditorCacheData(metaData.TempQueryEditorID) // expected to be present not deleted because ttl is double the collector interval
		if metaData.IsForLastXTime && present {
			rawDataMap = cacheData.(map[int]*models.MultiInstanceRawData)
		}
	}

	/*
		Step 3
		1. Initiate goroutines to call API for each time range caclulated
	*/
	wg := &sync.WaitGroup{}
	dataLenIdx := len(rawDataMap)
	for i := 0; i < len(timeRangeForApiCall) && response.Error == nil; i++ {
		wg.Add(1)
		go func(wg *sync.WaitGroup, rawDataLen int, from int64, to int64, metaData models.MetaData) backend.DataResponse {
			// Data is not present in the cache so fresh data needs to be fetched
			rawData, err := callRawDataAPI(&queryModel, pluginSettings, authSettings, from, to, metaData, logger)
			if err != nil {
				response.Error = err
				wg.Done()
				return response
			}
			rawDataMap[rawDataLen] = &rawData
			wg.Done()
			return response
		}(wg, dataLenIdx, timeRangeForApiCall[i].From, timeRangeForApiCall[i].To, metaData)
		dataLenIdx++
	}
	wg.Wait()
	// check for any errors
	if response.Error != nil {
		mainTaskWaitGroup.Done()
		return response
	}

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
	} else {
		mainTaskWaitGroup.Done()
		return response
	}

	/*
		Step 4
		1. Build frame from results of all API calls. Append results from subsequent API calls in accordance to time ranges.
		2. Get lastest timestamp of radwdata from instances
	*/
	var dataFrameMap = make(map[string]*data.Frame)
	logger.Info("actualNumberOfCallsBeforeAppend => ", actualNumberOfCallsBeforeAppend)
	for i := 0; i < len(rawDataMap); i++ {
		if actualNumberOfCallsBeforeAppend > 0 && i >= actualNumberOfCallsBeforeAppend {
			metaData.ApendRequest = true
		}
		dataFrameMap, metaData.MatchedInstances = BuildFrameFromMultiInstance(metaData.FrameId, &queryModel, &rawDataMap[i].Data, dataFrameMap, metaData, logger)
	}

	/*
		Step 5
		1. Check for Errors if any else store results in cache and return
	*/
	if !metaData.MatchedInstances && len(dataFrameMap) == 0 {
		response.Error = errors.New(constants.InstancesNotMatchingWithHosts)
	} else {
		for _, frame := range dataFrameMap {
			response.Frames = append(response.Frames, frame)
		}
		queryModel.FrameCacheTTLInSeconds = queryModel.CollectInterval + (constants.AdditionalFrameCacheTTLInMinutes * 60)
		// Add data to cache
		if metaData.IsCallFromQueryEditor {
			cache.StoreQueryEditorTempData(metaData.TempQueryEditorID, (constants.QueryEditorCacheTTLInMinutes * 60), rawDataMap)
			// Store updated data in frame cache. this data returned when entry is deleted from editor temp cache.
			// this avoids new API calls for frame cache
			cache.StoreFrame(metaData.FrameId, queryModel.FrameCacheTTLInSeconds, response.Frames)
			logger.Info("instanceWithLastRawDataEntryTimestamp => ", time.UnixMilli(cache.GetLastestRawDataEntryTimestamp(metaData.TempQueryEditorID)*1000))
		} else {
			cache.StoreFrame(metaData.FrameId, queryModel.FrameCacheTTLInSeconds, response.Frames)
			logger.Info("instanceWithLastRawDataEntryTimestamp => ", time.UnixMilli(cache.GetLastestRawDataEntryTimestamp(metaData.FrameId)*1000))
		}
	}
	mainTaskWaitGroup.Done()
	return response
}

// Gets fresh data by calling rest API
func callRawDataAPI(queryModel *models.QueryModel, pluginSettings *models.PluginSettings,
	authSettings *models.AuthSettings, from int64, to int64, metaData models.MetaData, logger log.Logger) (models.MultiInstanceRawData, error) {
	var rawData models.MultiInstanceRawData //nolint:exhaustivestruct

	fullPath := BuildURLReplacingQueryParams(constants.RawDataMultiInstanceReq, queryModel, from, to, metaData)

	logger.Info("Calling API  => ", pluginSettings.Path, fullPath)
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
