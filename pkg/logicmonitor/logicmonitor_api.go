package logicmonitor

import (
	"context"
	"encoding/json"
	"strconv"
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
	response := backend.DataResponse{} //nolint:exhaustivestruct

	// Unmarshal the JSON into our queryModel.
	var queryModel models.QueryModel

	response.Error = json.Unmarshal(query.JSON, &queryModel)
	if response.Error != nil || queryModel.DataPointSelected == nil {
		logger.Error(constants.ErrorUnmarshallingErrorData+"queryModel =>", response.Error)

		return response
	}
	logger.Info("queryModel.interpolatedQuery? => ", queryModel.IsQueryInterpolated)
	if queryModel.IsQueryInterpolated {
		requestURL := BuildURLReplacingQueryParams(constants.HostDataSourceReq, &queryModel, nil)
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
			response.Error = errors.New(constants.HostHasNoMatchingDataSource + queryModel.DataSourceSelected.Label)
			return response
		}
	}

	// Set the temp query Editor ID rame id
	tempQueryEditorID := getQueryEditorTempID(&queryModel, &query, pluginSettings)
	// Set the frame id
	frameID := getFrameID(&queryModel, tempQueryEditorID)

	ifCallFromQueryEditor := checkIfCallFromQueryEditor(&queryModel)
	logger.Info("Call is from Query Editor = ", ifCallFromQueryEditor, queryModel.CollectInterval, queryModel.LastQueryEditedTimeStamp)
	matchedInstances := false
	if ifCallFromQueryEditor {
		// Check if data is in temporary cache. user has recently updated panel,
		// Keeps data for datasource interval time from the last time user has updated query
		response, err := getFromQueryEditorTempCache(tempQueryEditorID, &queryModel, logger, matchedInstances)
		if !matchedInstances && len(response.Frames) == 0 {
			response.Error = errors.New("Instance are matching with selected host")
		}
		if err == nil {
			return response
		}
	} else {
		// Gets Data from local cache for the selected query.
		// currently data is present in frameCache and query is updated, it should not refer frameCache
		logger.Info("The Frame id is = ", frameID)
		response, err := getFromFrameCache(frameID, logger)
		if err == nil {
			return response
		}
	}

	// Data is not present in the cache so fresh data needs to be fetched
	rawData, err := callRawDataAPI(&queryModel, pluginSettings, authSettings, query, logger)
	if err != nil {
		response.Error = err
		logger.Error("Error calling rawData API ", err)

		return response
	}
	response = BuildFrameFromMultiInstance(&queryModel, &rawData.Data, matchedInstances)
	if !matchedInstances && len(response.Frames) == 0 {
		response.Error = errors.New("Instance are matching with selected host")
	}
	// Add data to cache
	if ifCallFromQueryEditor {
		cache.StoreQueryEditorTempData(tempQueryEditorID, queryModel.CollectInterval, rawData.Data)
		// Get updated data when entry is deleted from temp cache. this avoids old data in frame cache being returned
		// as there can be timerange change/query change that frame cache is not udpated yet
		cache.StoreFrame(frameID, queryModel.CollectInterval, response.Frames)
	} else {
		cache.StoreFrame(frameID, queryModel.CollectInterval, response.Frames)
	}

	return response
}

// This if block serves while updating query, temporarily stores results of rawdata for all instance and data points.
// that avoid rest calls while selecting multiple instances/datapoints
func getFromQueryEditorTempCache(uniqueID string, qm *models.QueryModel, logger log.Logger, matchedInstances bool) (backend.DataResponse, error) { //nolint:lll
	cacheData, present := cache.GetQueryEditorCacheData(uniqueID)
	if present {
		//todo remove the loggers
		logger.Info("From QueryEditorCache => FrameCache size = ", cache.GetFrameDataCount())
		logger.Info("From QueryEditorCache => QueryEditorCache size = ", cache.GetQueryEditorCacheDataCount())

		rawData := cacheData.(models.MultiInstanceData)
		return BuildFrameFromMultiInstance(qm, &rawData, matchedInstances), nil
	}

	return backend.DataResponse{}, errors.New(constants.DataNotPresentEditorCacheErrMsg)
}

// Gets Data from local cache for the selected query.
// The cache is used for the collector interval duration. Also data is stored only for the instances asked
func getFromFrameCache(uniqueID string, logger log.Logger) (backend.DataResponse, error) {
	response := backend.DataResponse{} //nolint:exhaustivestruct

	frameValue, framePresent := cache.GetData(uniqueID)
	if framePresent {
		//todo remove the loggers
		logger.Info("From FrameCache => FrameCache size = ", cache.GetFrameDataCount())
		logger.Info("From FrameCache => QueryEditorCache size = ", cache.GetQueryEditorCacheDataCount())

		response.Frames = frameValue.(data.Frames)

		return response, nil
	}

	logger.Error("Entry not exist in cache => ", uniqueID)

	return response, errors.New(constants.DataNotPresentCacheErrMsg)
}

// Gets fresh data by calling rest API
func callRawDataAPI(queryModel *models.QueryModel, pluginSettings *models.PluginSettings,
	authSettings *models.AuthSettings, query backend.DataQuery, logger log.Logger) (models.MultiInstanceRawData, error) {
	var rawData models.MultiInstanceRawData //nolint:exhaustivestruct

	fullPath := BuildURLReplacingQueryParams(constants.RawDataMultiInstanceReq, queryModel, &query)

	//todo remove the loggers
	logger.Info("API Call => FrameCache size = ", cache.GetFrameDataCount())
	logger.Info("API Call => QueryEditorCache size = ", cache.GetQueryEditorCacheDataCount())

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

func getQueryEditorTempID(queryModel *models.QueryModel, query *backend.DataQuery, pluginSettings *models.PluginSettings) string { //nolint:lll
	lastFromTimeUnixTruncated := UnixTruncateToNearestMinute(query.TimeRange.From, queryModel.CollectInterval)
	lastToTimeUnixTruncated := UnixTruncateToNearestMinute(query.TimeRange.To, queryModel.CollectInterval)

	return pluginSettings.Path + queryModel.TypeSelected + queryModel.GroupSelected.Label +
		queryModel.HostSelected.Label + queryModel.DataSourceSelected.Label +
		strconv.FormatInt(lastFromTimeUnixTruncated, 10) + strconv.FormatInt(lastToTimeUnixTruncated, 10)
}

func getFrameID(queryModel *models.QueryModel, tempQueryEditorID string) string { //nolint:lll
	return tempQueryEditorID + strconv.FormatInt(queryModel.LastQueryEditedTimeStamp, 10)
}

func checkIfCallFromQueryEditor(queryModel *models.QueryModel) bool {
	return (time.Now().UnixMilli()-queryModel.LastQueryEditedTimeStamp)/1000 < queryModel.CollectInterval
}
