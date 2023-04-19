package logicmonitor

import (
	"encoding/json"
	httpclient "github.com/grafana/grafana-logicmonitor-datasource-backend/pkg/httpclient"
	"strconv"
	"time"

	"github.com/grafana/grafana-logicmonitor-datasource-backend/pkg/cache"
	"github.com/grafana/grafana-logicmonitor-datasource-backend/pkg/constants"
	"github.com/grafana/grafana-logicmonitor-datasource-backend/pkg/models"
	utils "github.com/grafana/grafana-logicmonitor-datasource-backend/pkg/utils"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
)

func Query(santabaClient httpclient.SantabaClient,
	pluginContext backend.PluginContext, query backend.DataQuery) backend.DataResponse {
	response := backend.DataResponse{} //nolint:exhaustivestruct

	// Unmarshal the JSON into our queryModel.
	var queryModel models.QueryModel
	var metaData models.MetaData

	response.Error = json.Unmarshal(query.JSON, &queryModel)
	if response.Error != nil || queryModel.DataPointSelected == nil {
		santabaClient.Logger.Error(constants.ErrorUnmarshallingErrorData+"queryModel =>", response.Error)
		return response
	}
	santabaClient.Logger.Debug("queryModel => ", queryModel)
	// interpolatedQuery, when variable is added on dashboard, one variable on dashboard is hadled here. its considered to be host
	if queryModel.EnableHostVariableFeature {
		santabaClient.Logger.Debug("queryModel.interpolatedQuery? => ", queryModel.IsQueryInterpolated)
		if queryModel.IsQueryInterpolated {
			queryModel, response = cache.InterpolateHostDataSourceDetails(santabaClient, queryModel, response)
		}
	}

	metaData.EditMode = checkIfCallFromQueryEditor(&queryModel)
	metaData.Id, metaData.IsForLastXTime = getUniqueID(&queryModel, &query, santabaClient.PluginSettings, metaData)
	metaData.QueryId = getQueryId(&queryModel, &query, santabaClient.PluginSettings)
	if queryModel.EnableHistoricalData {
		if queryModel.EnableStrategicApiCallFeature {
			metaData.CacheTTLInSeconds = query.TimeRange.To.Unix() - query.TimeRange.From.Unix()
		} else {
			metaData.CacheTTLInSeconds = 120
		}
	} else {
		metaData.CacheTTLInSeconds = 60
	}
	santabaClient.Logger.Debug("metaData.CacheTTLInSeconds = ", metaData.CacheTTLInSeconds)
	metaData.InstanceSelectedMap = make(map[string]int)
	for i, v := range queryModel.InstanceSelected {
		metaData.InstanceSelectedMap[v.Label] = i
	}
	santabaClient.Logger.Debug("metaData ==> ", metaData)
	return GetData(query, queryModel, metaData, santabaClient, pluginContext)
	// go GetData(query, queryModel, metaData, authSettings, pluginSettings, pluginContext, logger)
	// finalData := make(map[int]*models.MultiInstanceRawData)
	// if data, ok := cache.GetData(metaData); ok {
	// 	if cachedData, ok := data.(*models.MultiInstanceRawData); ok {
	// 		finalData[0] = cachedData
	// 	}
	// }
	// if len(finalData) > 0 {
	// 	return processFinalData(queryModel, metaData, query.TimeRange.From.Unix(), query.TimeRange.To.Unix(), finalData, response, logger)
	// } else {
	// 	return response
	// }
}

func getUniqueID(queryModel *models.QueryModel, query *backend.DataQuery, pluginSettings *models.PluginSettings, metaData models.MetaData) (string, bool) { //nolint:lll
	if !queryModel.EnableStrategicApiCallFeature {
		//backword compatible
		return getIDForOneMinute(queryModel, query, pluginSettings, metaData)
	}
	if utils.UnixTruncateToNearestMinute(query.TimeRange.To.Unix(), 60) > (time.Now().Unix() - constants.LastXMunitesCheckForFrameIdCalculationInSec) { // LastXTime, return true in this case
		if metaData.EditMode {
			return getQueryId(queryModel, query, pluginSettings), true
		} else {
			return getQueryId(queryModel, query, pluginSettings) + strconv.FormatInt(queryModel.LastQueryEditedTimeStamp, 10), true
		}
	} else { // FixedTimeRange, returns false for the same
		if metaData.EditMode {
			return getQueryId(queryModel, query, pluginSettings), false
		} else {
			return getQueryId(queryModel, query, pluginSettings) + strconv.FormatInt(queryModel.LastQueryEditedTimeStamp, 10), false
		}
	}
}

func getQueryId(queryModel *models.QueryModel, query *backend.DataQuery, pluginSettings *models.PluginSettings) string {
	return pluginSettings.Path + queryModel.TypeSelected + queryModel.GroupSelected.Label +
		queryModel.HostSelected.Label + queryModel.DataSourceSelected.Label
}

func getIDForOneMinute(queryModel *models.QueryModel, query *backend.DataQuery, pluginSettings *models.PluginSettings, metaData models.MetaData) (string, bool) { //nolint:lll
	FromTimeUnixTruncated := utils.UnixTruncateToNearestMinute(query.TimeRange.From.Unix(), 60)
	ToTimeUnixTruncated := utils.UnixTruncateToNearestMinute(query.TimeRange.To.Unix(), 60)
	if metaData.EditMode {
		return getQueryId(queryModel, query, pluginSettings) +
			strconv.FormatInt(FromTimeUnixTruncated, 10) + strconv.FormatInt(ToTimeUnixTruncated, 10), true
	} else {
		return getQueryId(queryModel, query, pluginSettings) +
			strconv.FormatInt(FromTimeUnixTruncated, 10) + strconv.FormatInt(ToTimeUnixTruncated, 10) +
			strconv.FormatInt(queryModel.LastQueryEditedTimeStamp, 10), false
	}
}

func checkIfCallFromQueryEditor(queryModel *models.QueryModel) bool {
	return (time.Now().UnixMilli()-queryModel.LastQueryEditedTimeStamp)/1000 < constants.EditModeLastingSeconds
}
