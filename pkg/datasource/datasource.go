package plugin

import (
	"context"
	"encoding/json"
	"github.com/grafana/grafana-logicmonitor-datasource-backend/pkg/constants"
	"github.com/grafana/grafana-logicmonitor-datasource-backend/pkg/httpclient"
	"github.com/grafana/grafana-logicmonitor-datasource-backend/pkg/logicmonitor"
	"github.com/grafana/grafana-logicmonitor-datasource-backend/pkg/models"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/instancemgmt"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
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
		logger.Error("Error unmarshalling the Plugin Settings", err)

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

	resp, err := httpclient.Get(ds.PluginSettings, ds.AuthSettings, constants.DeviceDevicesPath, constants.DevicesSizeOnePath, ds.Logger) //nolint:lll
	if err != nil {
		healthRequest.Message = constants.HealthAPIErrMsg
		healthRequest.Status = backend.HealthStatusError

		return healthRequest, nil //nolint:nilerr
	}

	if resp.StatusCode == http.StatusServiceUnavailable ||
		resp.StatusCode == http.StatusInternalServerError ||
		resp.StatusCode == http.StatusBadRequest {
		healthRequest.Message = constants.HostUnreachableErrMsg
		healthRequest.Status = backend.HealthStatusError

		return healthRequest, nil
	}

	deviceData := models.DeviceData{} //nolint:exhaustivestruct

	//todo remove this
	ds.Logger.Info("The request was ", resp.Request.URL)
	ds.Logger.Info("The status code was ", resp.Status)
	resDump, err := httputil.DumpResponse(resp, true)
	if err != nil {
		ds.Logger.Error(err.Error())
	}
	ds.Logger.Info("HTTP Response in Datasource", resDump)

	bodyText, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		ds.Logger.Info("Error in ioutil.Readall => ", resp.Body)
		healthRequest.Message = "Error in ioutil.Readall"
		healthRequest.Status = backend.HealthStatusError

		ds.Logger.Error(constants.UnmarshallingErrMsg, err.Error)
		return healthRequest, nil //nolint:nilerr
	}

	json.Unmarshal(bodyText, &deviceData)
	//err = json.NewDecoder(bodyText).Decode(&deviceData)
	if err != nil {
		healthRequest.Message = constants.UnmarshallingErrMsg
		healthRequest.Status = backend.HealthStatusError

		ds.Logger.Error(constants.UnmarshallingErrMsg, err.Error)

		return healthRequest, nil //nolint:nilerr
	}

	ds.Logger.Debug("The healthcheck healthStatus code is  => ", deviceData.Status)

	if deviceData.Status == http.StatusOK {
		healthRequest.Status = backend.HealthStatusOk
		healthRequest.Message = constants.AuthSuccessMsg

		return healthRequest, nil
	}

	healthRequest.Status = backend.HealthStatusError

	if deviceData.Status == http.StatusBadRequest {
		healthRequest.Message = constants.InvalidTokenErrMsg + deviceData.Errmsg
		ds.Logger.Error("Invalid Token for Company or " + deviceData.Errmsg)
	} else {
		healthRequest.Message = constants.APIErrMsg + deviceData.Errmsg
		ds.Logger.Error(constants.APIErrMsg, deviceData.Errmsg)
	}

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

	resp, err := httpclient.Get(ds.PluginSettings, ds.AuthSettings, req.Path, req.URL, ds.Logger)
	if err != nil {
		ds.Logger.Info(" Error from server => ", err)

		return err
	}

	bodyText, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		ds.Logger.Info(" Error reading response => ", resp.Body)

		return err
	}

	return sender.Send(&backend.CallResourceResponse{ //nolint:exhaustivestruct
		Status: resp.StatusCode,
		Body:   []byte(bodyText), //nolint:unconvert
	})
}
