package logicmonitor

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"time"

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
		logger.Error("Error Unmarshaling queryModel = ", response.Error)
		return response
	}

	//Set the unique id
	uniqueID := getUniqueID(&queryModel, &query, pluginSettings)

	//Check if data is in temprary cache. user has recently updated panel,
	//Keeps data for datasource interval time from the last time user has updated query
	response, err := getFromTempCache(uniqueID, queryModel, pluginSettings, authSettings, query, logger)
	if err == nil {
		return response
	}

	//Gets Data from local cache for the selected query.
	response, err = getFromFrameCache(uniqueID, queryModel.CollectInterval, query, logger)
	if err == nil {
		return response
	}

	//datasource interval has been exhousted and fresh data needs to be fetched
	rawData, err := callRawDataAPI(queryModel, pluginSettings, authSettings, query, logger)
	if err != nil {
		response.Error = err
		logger.Error("Error calling rawData API ", err)
		return response
	}

	response = buildFrameFromMultiInstace(queryModel, rawData.Data)
	// Add data to cache
	cache.StoreFrame(uniqueID, queryModel.CollectInterval, query, response.Frames)
	return response
}

// This if block serves while updating query, temporarily stores results of rawdata for all instance and data points.
// that avoid rest calls while selecting multiple instances/datapoints
func getFromTempCache(uniqueId string, qm models.QueryModel, pluginSettings *models.PluginSettings, authSettings *models.AuthSettings,
	query backend.DataQuery, logger log.Logger) (backend.DataResponse, error) {
	lastTimeRange, lastTimeRangePresent := cache.GetLastTimeRange(uniqueId)
	logger.Info("")
	logger.Info("Is TimeRange Changed ? => ", !(lastTimeRangePresent && lastTimeRange == cache.GetCurrentTimeRange(query)))
	if (time.Now().UnixMilli()-qm.LastQueryEditedTimeStamp)/1000 < qm.CollectInterval &&
		lastTimeRangePresent && lastTimeRange == cache.GetCurrentTimeRange(query) {
		if !cache.IsTempRawDataPresent(uniqueId) {
			rawdata, err := callRawDataAPI(qm, pluginSettings, authSettings, query, logger)
			if err != nil {
				logger.Error("Error calling rawData API ", err)
				return backend.DataResponse{}, fmt.Errorf("no data")
			}
			response := buildFrameFromMultiInstace(qm, rawdata.Data)
			cache.StoreTemp(uniqueId, qm.CollectInterval, rawdata.Data)
			return response, nil
		}
		logger.Info("From TempCache => FrameCache size = ", cache.RawDataCount())
		logger.Info("From TempCache => TempRawDataCache size = ", cache.GetTempRawDataCount())
		rdata, _ := cache.GetTempRawData(uniqueId)
		rawdata := rdata.(models.MultiInstanceData)
		return buildFrameFromMultiInstace(qm, rawdata), nil
	}
	return backend.DataResponse{}, fmt.Errorf("no data")
}

// Gets Data from local cache for the selected query.
func getFromFrameCache(uniqueId string, collectInterval int64, query backend.DataQuery, logger log.Logger) (backend.DataResponse, error) {
	lastExecutedTime, lastExecutedTimePresent := cache.GetLastExecutedTime(uniqueId)
	lastTimeRange, lastTimeRangePresent := cache.GetLastTimeRange(uniqueId)
	response := backend.DataResponse{}
	if lastExecutedTimePresent &&
		(lastExecutedTime+(collectInterval*1000)) > time.Now().Unix() &&
		lastTimeRangePresent && lastTimeRange == cache.GetCurrentTimeRange(query) {
		frameValue, framePresent := cache.GetData(uniqueId)
		if framePresent {
			logger.Info("From FrameCache => FrameCache size = ", cache.RawDataCount())
			logger.Info("From FrameCache => TempRawDataCache size = ", cache.GetTempRawDataCount())
			response.Frames = frameValue.(data.Frames)
			return response, nil
		} else {
			logger.Error("Entry not exist in cache => ", uniqueId)
		}
	}
	return response, fmt.Errorf("no data")
}

// Gets fresh data by calling rest API
func callRawDataAPI(queryModel models.QueryModel, pluginSettings *models.PluginSettings, authSettings *models.AuthSettings,
	query backend.DataQuery, logger log.Logger) (models.MultiInstanceRawData, error) {
	var rawData models.MultiInstanceRawData //nolint:exhaustivestruct
	fullPath := BuildURLReplacingQueryParams(constants.RawDataMultiInstanceReq, &queryModel, &query)
	//todo
	logger.Debug("The full path is = ", fullPath)
	logger.Debug("Calling API for query = ", queryModel)
	logger.Debug("Cache size = ", cache.RawDataCount())
	logger.Info("API Call => FrameCache size = ", cache.RawDataCount())
	logger.Info("API Call => TempRawDataCache size = ", cache.GetTempRawDataCount())
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
	return pluginSettings.Path + queryModel.TypeSelected + queryModel.GroupSelected.Label + queryModel.HostSelected.Label + queryModel.DataSourceSelected.Label
}
