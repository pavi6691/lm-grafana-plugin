package logicmonitor

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/pkg/errors"

	"github.com/grafana/grafana-logicmonitor-datasource-backend/pkg/cache"
	"github.com/grafana/grafana-logicmonitor-datasource-backend/pkg/constants"
	"github.com/grafana/grafana-logicmonitor-datasource-backend/pkg/httpclient"
	"github.com/grafana/grafana-logicmonitor-datasource-backend/pkg/models"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
)

func Query(ctx context.Context, pluginSettings *models.PluginSettings, authSettings *models.AuthSettings,
	logger log.Logger, pluginContext backend.PluginContext, query backend.DataQuery) backend.DataResponse {
	response := backend.DataResponse{} //nolint:exhaustivestruct

	// Unmarshal the JSON into our queryModel.
	var queryModel models.QueryModel
	var metaData models.MetaData

	response.Error = json.Unmarshal(query.JSON, &queryModel)
	if response.Error != nil || queryModel.DataPointSelected == nil {
		logger.Error(constants.ErrorUnmarshallingErrorData+"queryModel =>", response.Error)
		return response
	}
	logger.Debug("queryModel => ", queryModel)
	// interpolatedQuery, when variable is added on dashboard, one variable on dashboard is hadled here. its considered to be host
	if queryModel.EnableHostVariableFeature {
		logger.Info("queryModel.interpolatedQuery? => ", queryModel.IsQueryInterpolated)
		hdsSelected, present := cache.GetHdsByHostAndDs(queryModel.HostSelected.Value, queryModel.DataSourceSelected.Ds)
		if queryModel.IsQueryInterpolated {
			if !present {
				requestURL := BuildURLReplacingQueryParams(constants.HostDataSourceReq, &queryModel, 0, 0, models.MetaData{})
				if requestURL == "" {
					logger.Error(constants.URLConfigurationErrMsg)
					return response
				}
				var respByte []byte
				respByte, response.Error = httpclient.Get(pluginSettings, authSettings, requestURL, constants.HostDataSourceReq, logger)
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
				if hdsReponse.Total == 1 {
					queryModel.HdsSelected = hdsReponse.Items[0].Id
					cache.StoreHds(queryModel.HostSelected.Value, queryModel.DataSourceSelected.Ds, queryModel.HdsSelected)
				} else if hdsReponse.Total > 1 {
					response.Error = errors.New(constants.MoreThanOneHostDataSources + queryModel.DataSourceSelected.Label)
					return response
				} else {
					response.Error = errors.New(fmt.Sprintf(constants.HostHasNoMatchingDataSource, queryModel.DataSourceSelected.Label))
					return response
				}
			} else {
				queryModel.HdsSelected = hdsSelected
			}
		}
	}

	metaData.TempQueryEditorID = getQueryEditorTempID(&queryModel, &query, pluginSettings)
	metaData.FrameId, metaData.IsForLastXTime = getFrameID(&queryModel, &query, pluginSettings)
	metaData.IsCallFromQueryEditor = checkIfCallFromQueryEditor(&queryModel)
	metaData.FrameCacheTTLInSeconds = queryModel.CollectInterval + (constants.AdditionalFrameCacheTTLInMinutes * 60)
	logger.Debug("tempQueryEditorID ==> ", metaData.TempQueryEditorID)
	logger.Debug("frameID ==> ", metaData.FrameId)
	logger.Debug("isForLastXTime ==> ", metaData.IsForLastXTime)

	return GetData(query, queryModel, metaData, authSettings, pluginSettings, pluginContext, logger)
}

func getQueryEditorTempID(queryModel *models.QueryModel, query *backend.DataQuery, pluginSettings *models.PluginSettings) string { //nolint:lll
	if !queryModel.EnableDataAppendFeature {
		//backword compatible
		return getUniqueID(queryModel, query, pluginSettings)
	}
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
	if !queryModel.EnableDataAppendFeature {
		//backword compatible
		return getUniqueID(queryModel, query, pluginSettings) + strconv.FormatInt(queryModel.LastQueryEditedTimeStamp, 10), true
	}
	if UnixTruncateToNearestMinute(query.TimeRange.To.Unix(), 60) > (time.Now().Unix() - constants.LastXMunitesCheckForFrameIdCalculationInSec) { // LastXTime, return true in this case
		return pluginSettings.Path + queryModel.TypeSelected + queryModel.GroupSelected.Label +
			queryModel.HostSelected.Label + queryModel.DataSourceSelected.Label + strconv.FormatInt(queryModel.LastQueryEditedTimeStamp, 10), true
	} else { // FixedTimeRange, returns false for the same
		return pluginSettings.Path + queryModel.TypeSelected + queryModel.GroupSelected.Label +
			queryModel.HostSelected.Label + queryModel.DataSourceSelected.Label + strconv.FormatInt(queryModel.LastQueryEditedTimeStamp, 10), false
	}
}

func getUniqueID(queryModel *models.QueryModel, query *backend.DataQuery, pluginSettings *models.PluginSettings) string { //nolint:lll
	lastFromTimeUnixTruncated := UnixTruncateToNearestMinute(query.TimeRange.From.Unix(), 60)
	lastToTimeUnixTruncated := UnixTruncateToNearestMinute(query.TimeRange.To.Unix(), 60)

	return pluginSettings.Path + queryModel.TypeSelected + queryModel.GroupSelected.Label +
		queryModel.HostSelected.Label + queryModel.DataSourceSelected.Label +
		strconv.FormatInt(lastFromTimeUnixTruncated, 10) + strconv.FormatInt(lastToTimeUnixTruncated, 10)
}

func checkIfCallFromQueryEditor(queryModel *models.QueryModel) bool {
	return (time.Now().UnixMilli()-queryModel.LastQueryEditedTimeStamp)/1000 < (constants.QueryEditorCacheTTLInMinutes * 60)
}
