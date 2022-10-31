package logicmonitor

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/pkg/errors"

	"github.com/grafana/grafana-logicmonitor-datasource-backend/pkg/cache"
	"github.com/grafana/grafana-logicmonitor-datasource-backend/pkg/constants"
	"github.com/grafana/grafana-logicmonitor-datasource-backend/pkg/httpclient"
	"github.com/grafana/grafana-logicmonitor-datasource-backend/pkg/models"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/grafana/grafana-plugin-sdk-go/data"
)

func Query(ctx context.Context, pluginSettings *models.PluginSettings, authSettings *models.AuthSettings,
	logger log.Logger, pluginContext backend.PluginContext, query backend.DataQuery) backend.DataResponse {
	logger.Info("")
	response := backend.DataResponse{} //nolint:exhaustivestruct

	// Unmarshal the JSON into our queryModel.
	var queryModel models.QueryModel
	var metaData models.MetaData

	response.Error = json.Unmarshal(query.JSON, &queryModel)
	if response.Error != nil || queryModel.DataPointSelected == nil {
		logger.Error(constants.ErrorUnmarshallingErrorData+"queryModel =>", response.Error)

		return response
	}

	// interpolatedQuery, when variable is added on dashboard, one variable on dashboard is hadled here. its considered to be host
	logger.Debug("queryModel.interpolatedQuery? => ", queryModel.IsQueryInterpolated)
	if queryModel.IsQueryInterpolated {
		requestURL := BuildURLReplacingQueryParams(constants.HostDataSourceReq, &queryModel, 0, 0, models.MetaData{})
		if requestURL == "" {
			logger.Error(constants.URLConfigurationErrMsg)
			return response
		}
		var respByte []byte
		respByte, response.Error = httpclient.Get(pluginSettings, authSettings, requestURL, logger)
		if response.Error != nil {
			logger.Error("Error from server => ", response.Error)
			return response
		}
		var hdsReponse models.HostDataSource
		response.Error = json.Unmarshal(respByte, &hdsReponse)
		if response.Error != nil {
			logger.Error(constants.ErrorUnmarshallingErrorData+"hdsReponse =>", response.Error.Error())
			return response
		}
		if hdsReponse.Data.Total == 1 {
			queryModel.HdsSelected = hdsReponse.Data.Items[0].Id
		} else if hdsReponse.Data.Total > 1 {
			response.Error = errors.New(constants.MoreThanOneHostDataSources + queryModel.DataSourceSelected.Label)
			return response
		} else {
			response.Error = errors.New(fmt.Sprintf(constants.HostHasNoMatchingDataSource, queryModel.DataSourceSelected.Label))
			return response
		}
	}

	// Set the temp query Editor ID rame id
	metaData.TempQueryEditorID = getQueryEditorTempID(&queryModel, &query, pluginSettings)
	// Set the frame id
	metaData.FrameId, metaData.IsForLastXTime = getFrameID(&queryModel, &query, pluginSettings)
	metaData.IsCallFromQueryEditor = checkIfCallFromQueryEditor(&queryModel)
	logger.Debug("tempQueryEditorID ==> ", metaData.TempQueryEditorID)
	logger.Debug("frameID ==> ", metaData.FrameId)
	logger.Debug("isForLastXTime ==> ", metaData.IsForLastXTime)
	if metaData.IsCallFromQueryEditor {
		/*
			Check if data is in temporary cache. user has recently updated panel,
			Keeps data for datasource interval time from the last time user has updated query
		*/
		var err error
		metaData.ActualNumberOfCallsBeforeAppend = len(GetTimeRanges(query.TimeRange.From.Unix(), query.TimeRange.To.Unix(), queryModel.CollectInterval, metaData, logger))
		response, metaData.InstanceWithLastRawDataEntryTimestamp, err = GetFromQueryEditorTempCache(&queryModel, metaData, logger)
		if err != nil && err.Error() == constants.InstancesNotMatchingWithHosts {
			response.Error = err
			return response
		}
		if err == nil {
			// store updated data from queryEditor temp cache. this avoids new API calls for frame cache
			cache.StoreFrame(metaData.FrameId, queryModel.FrameCacheTTLInSeconds, response.Frames)
			return response
		}
	} else {
		response, err := GetFromFrameCache(metaData.FrameId)
		if err == nil {
			/*
				fixed time range have the same data always. so return it from cache.
				 if for last x time and wait time is yet to over then return it from cache else call API for new data and append it
			*/
			waitSec := GetWaitTimeInSec(metaData.FrameId, queryModel.CollectInterval)
			if !metaData.IsForLastXTime || waitSec > 0 {
				logger.Info("From FrameCache : Remaining seconds for new API call => ", waitSec)
				return response
			} else {
				/*
					wait time is over and no recent update on query. append new raw data entry to framedata in cache.
					remove old entries as same number as new to match time range
				*/
				logger.Info(constants.CallApiAndAppendToFrameCache)
				metaData.InstanceWithLastRawDataEntryTimestamp = cache.GetLastestRawDataEntryTimestamp(metaData.FrameId)
			}
		} else if err.Error() == constants.DataNotPresentCacheErrMsg {
			/*
				As per TTL policy, entry may have been deleted from frameCache as query is not executed for long time.
				this happens when for long time, dashboard is not active or on query editor query is not executed
				reset this Timestamp. this will reload whole data as per time range in query
			*/
			metaData.InstanceWithLastRawDataEntryTimestamp = 0
		} else {
			logger.Error("Error When getting data from FrameCache, size is = ", cache.GetFrameDataCount(), err)
			response.Error = err
			return response
		}
	}
	waitGroup := &sync.WaitGroup{}
	waitGroup.Add(1)
	response = OldAndNewDataProcessorTask(query, queryModel, metaData, authSettings, pluginSettings, waitGroup, logger)
	waitGroup.Wait()
	// response, response.Error = GetFromFrameCache(frameID)
	return response
}

// This if block serves while updating query, temporarily stores results of rawdata for all instance and data points.
// that avoid rest calls while selecting multiple instances/datapoints
func GetFromQueryEditorTempCache(qm *models.QueryModel, metaData models.MetaData, logger log.Logger) (backend.DataResponse, int64, error) { //nolint:lll
	response := backend.DataResponse{}
	cacheData, present := cache.GetQueryEditorCacheData(metaData.TempQueryEditorID)
	var matchedInstances bool
	tempMap := make(map[string]*data.Frame)
	if present {
		rawDataMap := cacheData.(map[int]*models.MultiInstanceRawData)
		for i := 0; i < len(rawDataMap); i++ {
			if metaData.ActualNumberOfCallsBeforeAppend > 0 && i >= metaData.ActualNumberOfCallsBeforeAppend {
				metaData.ApendRequest = true
			}
			tempMap, matchedInstances = BuildFrameFromMultiInstance(metaData.TempQueryEditorID, qm, &rawDataMap[i].Data, tempMap, metaData, logger)
		}
		waitSec := GetWaitTimeInSec(metaData.TempQueryEditorID, qm.CollectInterval)
		if !matchedInstances && len(tempMap) == 0 {
			return response, cache.GetLastestRawDataEntryTimestamp(metaData.TempQueryEditorID), errors.New(constants.InstancesNotMatchingWithHosts)
		} else if !metaData.IsForLastXTime || waitSec > 0 {
			for _, frame := range tempMap {
				response.Frames = append(response.Frames, frame)
			}
			logger.Info("From QueryEditorCache : Remaining seconds for new API call  => ", waitSec)
			return response, cache.GetLastestRawDataEntryTimestamp(metaData.TempQueryEditorID), nil
		}
		logger.Info(constants.CallApiAndAppendToEditorCache)
		return backend.DataResponse{}, cache.GetLastestRawDataEntryTimestamp(metaData.TempQueryEditorID), errors.New(constants.CallApiAndAppendToEditorCache)
	} else {
		logger.Info(constants.DataNotPresentEditorCacheErrMsg)
		return backend.DataResponse{}, 0, errors.New(constants.DataNotPresentEditorCacheErrMsg)
	}
}

/*
Gets Data from local cache for the selected query.
The cache is used for the collector interval duration. Also data is stored only for the instances and dps selected
if there is any error from API response / processing an error is stored in differrent cache which is first cheked before sending actual data
*/
func GetFromFrameCache(uniqueID string) (backend.DataResponse, error) {
	response := backend.DataResponse{} //nolint:exhaustivestruct

	frameValue, framePresent := cache.GetData(uniqueID)
	if framePresent {
		if df, ok := frameValue.(data.Frames); ok {
			response.Frames = df
			return response, nil
		} else if er, ok := frameValue.(error); ok {
			return response, er
		} else {
			return response, errors.New(constants.InvalidFormatOfDataInFrameCache)
		}
	} else {
		return response, errors.New(constants.DataNotPresentCacheErrMsg)
	}
}

func getQueryEditorTempID(queryModel *models.QueryModel, query *backend.DataQuery, pluginSettings *models.PluginSettings) string { //nolint:lll
	if UnixTruncateToNearestMinute(query.TimeRange.To.Unix(), 60) > (time.Now().Unix() - constants.LastXMunitesCheckForFrameIdCalculationInSec) { // LastXTime, return true in this case
		return pluginSettings.Path + queryModel.TypeSelected + queryModel.GroupSelected.Label +
			queryModel.HostSelected.Label + queryModel.DataSourceSelected.Label +
			strconv.FormatInt(query.TimeRange.To.Unix()-query.TimeRange.From.Unix(), 10)
	} else { // FixedTimeRange, returns false for the same
		return pluginSettings.Path + queryModel.TypeSelected + queryModel.GroupSelected.Label +
			queryModel.HostSelected.Label + queryModel.DataSourceSelected.Label +
			strconv.FormatInt(query.TimeRange.From.Unix(), 10) + strconv.FormatInt(query.TimeRange.To.Unix(), 10)
	}

}

func getFrameID(queryModel *models.QueryModel, query *backend.DataQuery, pluginSettings *models.PluginSettings) (string, bool) { //nolint:lll
	lastInterval := query.TimeRange.To.Unix() - query.TimeRange.From.Unix()
	if UnixTruncateToNearestMinute(query.TimeRange.To.Unix(), 60) > (time.Now().Unix() - constants.LastXMunitesCheckForFrameIdCalculationInSec) { // LastXTime, return true in this case
		return pluginSettings.Path + queryModel.TypeSelected + queryModel.GroupSelected.Label +
			queryModel.HostSelected.Label + queryModel.DataSourceSelected.Label + strconv.FormatInt(queryModel.LastQueryEditedTimeStamp, 10) +
			strconv.FormatInt(lastInterval, 10), true
	} else { // FixedTimeRange, returns false for the same
		return pluginSettings.Path + queryModel.TypeSelected + queryModel.GroupSelected.Label +
			queryModel.HostSelected.Label + queryModel.DataSourceSelected.Label + strconv.FormatInt(queryModel.LastQueryEditedTimeStamp, 10) +
			strconv.FormatInt(query.TimeRange.From.Unix(), 10) + strconv.FormatInt(query.TimeRange.To.Unix(), 10), false
	}
}

func checkIfCallFromQueryEditor(queryModel *models.QueryModel) bool {
	return (time.Now().UnixMilli()-queryModel.LastQueryEditedTimeStamp)/1000 < (constants.QueryEditorCacheTTLInMinutes * 60)
}
