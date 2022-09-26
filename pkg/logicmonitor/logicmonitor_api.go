package logicmonitor

import (
	"context"
	"encoding/json"
	"github.com/grafana/grafana-logicmonitor-datasource-backend/pkg/cache"
	"github.com/grafana/grafana-logicmonitor-datasource-backend/pkg/constants"
	"github.com/grafana/grafana-logicmonitor-datasource-backend/pkg/httpclient"
	"github.com/grafana/grafana-logicmonitor-datasource-backend/pkg/models"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/grafana/grafana-plugin-sdk-go/data"
	"io/ioutil"
	"strconv"
	"time"
)

func Query(ctx context.Context, pluginSettings *models.PluginSettings, authSettings *models.AuthSettings, logger log.Logger,
	pluginContext backend.PluginContext, query backend.DataQuery) backend.DataResponse {
	response := backend.DataResponse{} //nolint:exhaustivestruct

	// Unmarshal the JSON into our queryModel.
	var queryModel models.QueryModel
	response.Error = json.Unmarshal([]byte(query.JSON), &queryModel)

	if response.Error != nil || queryModel.DataPointSelected == nil {
		return response
	}

	//Set the unique id
	queryModel.UniqueID = getUniqueID(&queryModel, &query)
	logger.Info("The UniqueID = " + queryModel.UniqueID)

	lastExecutedTime, lastExecutedTimePresent := cache.GetLastExecutedTime(queryModel.UniqueID)
	timeRangeChanged, timeRangeChangePresent := cache.GetTimeRangeChanged(queryModel.UniqueID)

	var lastExecutedTimeInt = int64(0)

	if lastExecutedTimePresent {
		ok := false

		lastExecutedTimeInt, ok = lastExecutedTime.(int64)
		if !ok {
			logger.Error("Last executed time cannot be asserted into int64")
		}
	}

	if lastExecutedTimePresent &&
		(lastExecutedTimeInt+(queryModel.CollectInterval)) > time.Now().Unix() &&
		timeRangeChangePresent && timeRangeChanged == query.TimeRange.Duration() {
		frameValue, framePresent := cache.GetData(queryModel.UniqueID)
		if framePresent {
			response.Frames = append(response.Frames, frameValue.(*data.Frame))
		} else {
			logger.Debug("Entry not exist in cache  => ", queryModel.UniqueID)
		}

		return response
	}

	fullPath := BuildURLReplacingQueryParams(constants.RawDataReq, &queryModel, &query)

	//todo
	logger.Debug("The full path is = ", fullPath)
	logger.Debug("Calling API for query = ", queryModel)
	logger.Debug("Cache size = ", cache.RawDataCount())

	resp, err := httpclient.Get(pluginSettings, authSettings, fullPath, logger)
	if err != nil {
		response.Error = err
		logger.Error(" Error from server => ", response.Error)

		return response
	}

	bodyText, err := ioutil.ReadAll(resp.Body)
	if err != nil || resp.StatusCode != 200 {
		logger.Info(" Error reading response => ", resp.Body)

		return response
	}

	rawData := models.RawData{} //nolint:exhaustivestruct

	response.Error = json.Unmarshal(bodyText, &rawData)
	if response.Error != nil {
		logger.Info("Error Unmarshalling raw-data => ", response.Error)

		return response
	}

	frame := BuildFrame(queryModel.DataPointSelected, rawData.Data)
	// add the frames to the response.
	response.Frames = append(response.Frames, frame)
	// Add data to cache
	cache.Store(queryModel, query, frame)

	return response
}

func getUniqueID(queryModel *models.QueryModel, query *backend.DataQuery) string {
	unixTruncateToMinute := UnixTruncateToMinute(query.TimeRange.From.Unix())
	return strconv.FormatInt(queryModel.HdsSelected, 10) + queryModel.InstanceSelected[0].Value + strconv.FormatInt(unixTruncateToMinute, 10)
}
