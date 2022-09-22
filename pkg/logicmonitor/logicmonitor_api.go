package logicmonitor

import (
	"context"
	"encoding/json"
	"github.com/grafana/grafana-logicmonitor-datasource-backend/pkg/cache"
	plugin "github.com/grafana/grafana-logicmonitor-datasource-backend/pkg/datasource"
	"github.com/grafana/grafana-logicmonitor-datasource-backend/pkg/httpClient"
	"github.com/grafana/grafana-logicmonitor-datasource-backend/pkg/models"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data"
	"io/ioutil"
	"time"
)

func Query(ctx context.Context, ds *plugin.LogicmonitorDataSource, pluginContext backend.PluginContext, query backend.DataQuery) backend.DataResponse {
	response := backend.DataResponse{}
	// Unmarshal the JSON into our queryModel.
	var queryModel models.QueryModel
	response.Error = json.Unmarshal([]byte(query.JSON), &queryModel)
	if response.Error != nil || queryModel.DataPointSelected == nil {
		return response
	}

	value, present := cache.GetLastExecutedTime(queryModel.UniqueID)
	tvalue, tPresent := cache.GetTimeRangeChanged(queryModel.UniqueID)
	if present && (value.(int64)+(queryModel.CollectInterval*1000)) > time.Now().UnixMilli() && tPresent && tvalue == query.TimeRange.Duration() {
		frameValue, framePresent := cache.GetData(queryModel.UniqueID)
		if framePresent {
			response.Frames = append(response.Frames, frameValue.(*data.Frame))
		} else {
			ds.Logger.Error("Entry not exist in cache  => ", queryModel.UniqueID)
		}
		return response
	}

	fullPath := BuildFullPath(&queryModel, &query)
	resourcePath := BuildResourcePath(&queryModel)
	ds.Logger.Info("The full path is = ", fullPath)
	ds.Logger.Info("The resourcePath path is = ", resourcePath)

	ds.Logger.Info("Calling API for query = ", queryModel)
	ds.Logger.Info("Cache size = ", cache.RawDataCount())

	resp, err := httpclient.Get(ds.PluginSettings.AccessID, ds.AuthSettings.AccessKey, ds.AuthSettings.BearerToken, resourcePath, fullPath, ds.PluginSettings.Path, ds.PluginSettings.Version)
	if err != nil {
		ds.Logger.Error(" Error from server => ", resp.Body)
		response.Error = err
		return response
	}

	bodyText, err := ioutil.ReadAll(resp.Body)
	if err != nil || resp.StatusCode != 200 {
		ds.Logger.Info(" Error reading response => ", resp.Body)
		return response
	}

	rawdata := models.RawData{}
	response.Error = json.Unmarshal(bodyText, &rawdata)
	if response.Error != nil {
		ds.Logger.Info("Error Unmarshaling rawdata => ", response.Error)
		return response
	}
	frame := BuildFrame(queryModel.DataPointSelected, rawdata.Data)
	// add the frames to the response.
	response.Frames = append(response.Frames, frame)

	cache.Store(queryModel, query, frame)

	return response
}
