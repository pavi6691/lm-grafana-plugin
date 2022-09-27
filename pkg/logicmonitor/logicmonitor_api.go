package logicmonitor

import (
	"context"
	"encoding/json"
	"io/ioutil"
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

func Query(ctx context.Context, pluginSettings *models.PluginSettings, authSettings *models.AuthSettings, logger log.Logger,
	pluginContext backend.PluginContext, query backend.DataQuery) backend.DataResponse {
	response := backend.DataResponse{} //nolint:exhaustivestruct

	// Unmarshal the JSON into our queryModel.
	var queryModel models.QueryModel
	response.Error = json.Unmarshal([]byte(query.JSON), &queryModel)
	if response.Error != nil || queryModel.DataPointSelected == nil {
		logger.Error("Error Unmarshalling queryModel = ", response.Error)
		return response
	}

	//Set the unique id
	uniqueID := getUniqueID(&queryModel, &query, pluginSettings)
	logger.Info("The Unique id is = ", uniqueID)

	ifCallFromQueryEditor := checkIfCallFromQueryEditor(&queryModel)
	//Check if data is in temporary cache. user has recently updated panel,
	//Keeps data for datasource interval time from the last time user has updated query
	response, err := getFromQueryEditorTempCache(uniqueID, &queryModel, logger)
	if err == nil {
		return response
	}

	//Gets Data from local cache for the selected query.
	if !ifCallFromQueryEditor {
		response, err = getFromFrameCache(uniqueID, logger)
		if err == nil {
			return response
		}
	}

	//Data is not present in the cache so fresh data needs to be fetched
	rawData, err := callRawDataAPI(&queryModel, pluginSettings, authSettings, query, logger)
	if err != nil {
		response.Error = err
		logger.Error("Error calling rawData API ", err)
		return response
	}

	response = BuildFrameFromMultiInstance(&queryModel, &rawData.Data)
	// Add data to cache
	if ifCallFromQueryEditor {
		cache.StoreQueryEditorTempData(uniqueID, queryModel.CollectInterval, rawData.Data)
	} else {
		cache.StoreFrame(uniqueID, queryModel.CollectInterval, response.Frames)
	}

	return response
}

// This if block serves while updating query, temporarily stores results of rawdata for all instance and data points.
// that avoid rest calls while selecting multiple instances/datapoints
func getFromQueryEditorTempCache(uniqueId string, qm *models.QueryModel, logger log.Logger) (backend.DataResponse, error) {
	cacheData, present := cache.GetQueryEditorCacheData(uniqueId)
	if present {
		//todo remove the loggers
		logger.Info("From QueryEditorCache => FrameCache size = ", cache.GetFrameDataCount())
		logger.Info("From QueryEditorCache => QueryEditorCache size = ", cache.GetQueryEditorCacheDataCount())
		rawData := cacheData.(models.MultiInstanceData)
		return BuildFrameFromMultiInstance(qm, &rawData), nil
	}
	return backend.DataResponse{}, errors.New(constants.DataNotPresentEditorCacheErrMsg)
}

// Gets Data from local cache for the selected query.
// The cache is used for the collector interval duration. Also data is stored only for the instances asked
func getFromFrameCache(uniqueId string, logger log.Logger) (backend.DataResponse, error) {
	response := backend.DataResponse{}
	frameValue, framePresent := cache.GetData(uniqueId)
	if framePresent {
		//todo remove the loggers
		logger.Info("From FrameCache => FrameCache size = ", cache.GetFrameDataCount())
		logger.Info("From FrameCache => QueryEditorCache size = ", cache.GetQueryEditorCacheDataCount())
		response.Frames = frameValue.(data.Frames)
		return response, nil
	} else {
		logger.Error("Entry not exist in cache => ", uniqueId)
	}
	return response, errors.New(constants.DataNotPresentCacheErrMsg)
}

// Gets fresh data by calling rest API
func callRawDataAPI(queryModel *models.QueryModel, pluginSettings *models.PluginSettings, authSettings *models.AuthSettings,
	query backend.DataQuery, logger log.Logger) (models.MultiInstanceRawData, error) {
	var rawData models.MultiInstanceRawData //nolint:exhaustivestruct
	fullPath := BuildURLReplacingQueryParams(constants.RawDataMultiInstanceReq, queryModel, &query)
	//todo remove the loggers
	logger.Debug("The full path is = ", fullPath)
	logger.Debug("Calling API for query = ", queryModel)
	logger.Debug("Cache size = ", cache.GetFrameDataCount())
	logger.Info("API Call => FrameCache size = ", cache.GetFrameDataCount())
	logger.Info("API Call => QueryEditorCache size = ", cache.GetQueryEditorCacheDataCount())
	resp, err := httpclient.Get(pluginSettings, authSettings, fullPath, logger)
	if err != nil {
		logger.Error("Error from server => ", err)
		return rawData, err
	}

	bodyText, err := ioutil.ReadAll(resp.Body)
	if err != nil || resp.StatusCode != 200 {
		logger.Error("Error reading response => ", resp.Body)
		return rawData, err
	}

	err = json.Unmarshal(bodyText, &rawData)
	if err != nil {
		logger.Error("Error Unmarshalling raw-data => ", err)
		return rawData, err
	}
	return rawData, nil
}

func getUniqueID(queryModel *models.QueryModel, query *backend.DataQuery, pluginSettings *models.PluginSettings) string {
	lastFromTimeUnixTruncated := UnixTruncateToNearestMinute(query.TimeRange.From, queryModel.CollectInterval)
	lastToTimeUnixTruncated := UnixTruncateToNearestMinute(query.TimeRange.To, queryModel.CollectInterval)
	return pluginSettings.Path + queryModel.TypeSelected + queryModel.GroupSelected.Label +
		queryModel.HostSelected.Label + queryModel.DataSourceSelected.Label +
		strconv.FormatInt(lastFromTimeUnixTruncated, 10) + strconv.FormatInt(lastToTimeUnixTruncated, 10)
}

func checkIfCallFromQueryEditor(queryModel *models.QueryModel) bool {
	return (time.Now().UnixMilli()-queryModel.LastQueryEditedTimeStamp)/1000 < queryModel.CollectInterval
}
