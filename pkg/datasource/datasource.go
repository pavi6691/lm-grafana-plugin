package datasource

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/grafana/grafana-logicmonitor-datasource-backend/pkg/constants"
	"github.com/grafana/grafana-logicmonitor-datasource-backend/pkg/httpclient"
	"github.com/grafana/grafana-logicmonitor-datasource-backend/pkg/logicmonitor"
	"github.com/grafana/grafana-logicmonitor-datasource-backend/pkg/models"
	utils "github.com/grafana/grafana-logicmonitor-datasource-backend/pkg/utils"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/instancemgmt"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
)

var (
	_ backend.QueryDataHandler      = (*LogicmonitorDataSource)(nil)
	_ backend.CheckHealthHandler    = (*LogicmonitorDataSource)(nil)
	_ instancemgmt.InstanceDisposer = (*LogicmonitorDataSource)(nil)
)

type LogicmonitorDataSource struct {
	dsInfo         *backend.DataSourceInstanceSettings
	Logger         log.Logger
	PluginSettings *models.PluginSettings
	AuthSettings   *models.AuthSettings
}

func LogicmonitorBackendDataSource(dsSettings backend.DataSourceInstanceSettings) (instancemgmt.Instance, error) {
	logger := log.New()
	logger.Debug("Initializing new data source instance")

	var pluginSettings models.PluginSettings

	err := json.Unmarshal(dsSettings.JSONData, &pluginSettings)
	if err != nil {
		logger.Error(constants.ErrorUnmarshallingErrorData+"Plugin Settings =>", err)

		return nil, err //nolint:wrapcheck
	}

	return &LogicmonitorDataSource{
		dsInfo:         &dsSettings,
		Logger:         logger,
		PluginSettings: &pluginSettings,
		AuthSettings: &models.AuthSettings{
			AccessKey:   dsSettings.DecryptedSecureJSONData[constants.AccessKey],
			BearerToken: dsSettings.DecryptedSecureJSONData[constants.BearerToken],
		},
	}, nil
}

// Dispose here tells plugin SDK that plugin wants to clean up resources when a new instance
// created. As soon as datasource settings change detected by SDK old datasource instance will
// be disposed and a new one will be created using LogicmonitorBackendDataSource factory function.
func (ds *LogicmonitorDataSource) Dispose() {
	// Clean up datasource instance resources.
}

// QueryData handles multiple queries and returns multiple responses.
// req contains the queries []DataQuery (where each query contains RefID as a unique identifier).
// The QueryDataResponse contains a map of RefID to the response for each query, and each response
// contains Frames ([]*Frame).

func (ds *LogicmonitorDataSource) QueryData(ctx context.Context, req *backend.QueryDataRequest) (*backend.QueryDataResponse, error) { //nolint:lll
	// create response struct
	response := backend.NewQueryDataResponse()
	// loop over queries and execute them individually.
	for _, q := range req.Queries {
		res := logicmonitor.Query(ctx, ds.PluginSettings, ds.AuthSettings, ds.Logger, req.PluginContext, q)

		// save the response in a hashmap
		// based on with RefID as identifier
		response.Responses[q.RefID] = res
	}

	return response, nil
}

// CheckHealth handles health checks sent from Grafana to the plugin.
// The main use case for these health checks is the test button on the
// datasource configuration page which allows users to verify that
// a datasource is working as expected.
func (ds *LogicmonitorDataSource) CheckHealth(_ context.Context, req *backend.CheckHealthRequest) (*backend.CheckHealthResult, error) { //nolint:lll
	healthRequest := ds.validatePluginSettings(ds.Logger)
	if healthRequest.Status == backend.HealthStatusError {
		return healthRequest, nil
	}

	requestURL := utils.BuildURLReplacingQueryParams(constants.HealthCheckReq, nil, 0, 0, models.MetaData{})
	if requestURL == "" {
		healthRequest.Message = constants.HealthAPIURLErrMsg
		healthRequest.Status = backend.HealthStatusError

		return healthRequest, nil
	}

	respByte, err := httpclient.Get(ds.PluginSettings, ds.AuthSettings, requestURL, constants.HealthCheckReq, ds.Logger)
	if err != nil {
		healthRequest.Message = err.Error()
		healthRequest.Status = backend.HealthStatusError

		return healthRequest, nil //nolint:nilerr
	}
	var res models.ErrResponse
	err = json.Unmarshal(respByte, &res)
	if err != nil {
		ds.Logger.Error(constants.ErrorUnmarshallingErrorData+"ErrResponse =>", err)
		healthRequest.Message = err.Error()
		healthRequest.Status = backend.HealthStatusError
		return healthRequest, nil //nolint:nilerr
	}

	if res.Errmsg != "" && res.Errmsg != "OK" {
		healthRequest.Message = res.Errmsg
		healthRequest.Status = backend.HealthStatusError
		return healthRequest, nil //nolint:nilerr
	}

	healthRequest.Status = backend.HealthStatusOk
	healthRequest.Message = constants.AuthSuccessMsg
	return healthRequest, nil
}

func (ds *LogicmonitorDataSource) validatePluginSettings(logger log.Logger) *backend.CheckHealthResult {
	checkHealthResult := &backend.CheckHealthResult{} //nolint:exhaustivestruct
	checkHealthResult.Status = backend.HealthStatusError

	if ds.PluginSettings.Path == "" {
		checkHealthResult.Message = constants.NoCompanyNameEnteredErrMsg
		logger.Error(constants.NoCompanyNameEnteredErrMsg)

		return checkHealthResult
	}

	if !ds.PluginSettings.IsLMV1Enabled && !ds.PluginSettings.IsBearerEnabled {
		checkHealthResult.Message = constants.NoAuthenticationErrMsg
		logger.Error(constants.NoAuthenticationErrMsg)

		return checkHealthResult
	}

	if ds.PluginSettings.IsBearerEnabled && ds.AuthSettings.BearerToken == "" {
		checkHealthResult.Message = constants.BearerTokenEmptyErrMsg
		logger.Error(constants.BearerTokenEmptyErrMsg)

		return checkHealthResult
	}

	if ds.PluginSettings.IsLMV1Enabled {
		if ds.AuthSettings.AccessKey == "" {
			checkHealthResult.Message = constants.AccessKeyEmptyErrMsg
			logger.Error(constants.AccessKeyEmptyErrMsg)

			return checkHealthResult
		}

		if ds.PluginSettings.AccessID == "" {
			checkHealthResult.Message = constants.AccessIDEmptyErrMsg
			logger.Error(constants.AccessIDEmptyErrMsg)

			return checkHealthResult
		}
	}

	checkHealthResult.Status = backend.HealthStatusOk
	checkHealthResult.Message = ""

	return checkHealthResult
}

func (ds *LogicmonitorDataSource) CallResource(ctx context.Context, req *backend.CallResourceRequest, sender backend.CallResourceResponseSender) error { //nolint:lll
	var queryModel models.QueryModel

	err := json.Unmarshal(req.Body, &queryModel)
	if err != nil {
		ds.Logger.Error(constants.ErrorUnmarshallingErrorData+"QueryModel =>", err.Error())

		return sender.Send(&backend.CallResourceResponse{ //nolint:wrapcheck,exhaustivestruct
			Status: http.StatusInternalServerError,
			Body:   []byte(fmt.Sprintf(constants.InternalServerErrorJsonErrMessage, constants.ErrorUnmarshallingErrorData+"QueryModel")),
		})
	}

	requestURL := utils.BuildURLReplacingQueryParams(req.Path, &queryModel, 0, 0, models.MetaData{})
	if requestURL == "" {
		ds.Logger.Error(constants.URLConfigurationErrMsg)

		return sender.Send(&backend.CallResourceResponse{ //nolint:wrapcheck,exhaustivestruct
			Status: http.StatusInternalServerError,
			Body:   []byte(fmt.Sprintf(constants.InternalServerErrorJsonErrMessage, constants.URLConfigurationErrMsg)),
		})
	}

	respByte, err := httpclient.Get(ds.PluginSettings, ds.AuthSettings, requestURL, req.Path, ds.Logger)
	if err != nil {
		ds.Logger.Info(" Error from server => ", err)

		return sender.Send(&backend.CallResourceResponse{ //nolint:wrapcheck,exhaustivestruct
			Status: http.StatusInternalServerError,
			Body:   []byte(fmt.Sprintf(constants.InternalServerErrorJsonErrMessage, err.Error())),
		})
	}

	return sender.Send(&backend.CallResourceResponse{ //nolint:wrapcheck,exhaustivestruct
		Status: http.StatusOK,
		Body:   respByte, //nolint:unconvert
	})
}
