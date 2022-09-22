package plugin

import (
	"context"
	"encoding/json"
	"github.com/grafana/grafana-logicmonitor-datasource-backend/pkg/constants"
	httpClient "github.com/grafana/grafana-logicmonitor-datasource-backend/pkg/httpClient"
	"github.com/grafana/grafana-logicmonitor-datasource-backend/pkg/logicmonitor"
	"github.com/grafana/grafana-logicmonitor-datasource-backend/pkg/models"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/instancemgmt"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"io/ioutil"
)

// Make sure LogicmonitorDataSource implements required interfaces. This is important to do
// since otherwise we will only get a not implemented error response from plugin in
// runtime. In this example datasource instance implements backend.QueryDataHandler,
// backend.CheckHealthHandler, backend.StreamHandler interfaces. Plugin should not
// implement all these interfaces - only those which are required for a particular task.
// For example if plugin does not need streaming functionality then you are free to remove
// methods that implement backend.StreamHandler. Implementing instancemgmt.InstanceDisposer
// is useful to clean up resources used by previous datasource instance when a new datasource
// instance created upon datasource settings changed.
var (
	_ backend.QueryDataHandler   = (*LogicmonitorDataSource)(nil)
	_ backend.CheckHealthHandler = (*LogicmonitorDataSource)(nil)
	//_ backend.StreamHandler         = (*LogicmonitorDataSource)(nil)
	_ instancemgmt.InstanceDisposer = (*LogicmonitorDataSource)(nil)
)

type LogicmonitorDataSource struct {
	dsInfo         *backend.DataSourceInstanceSettings
	Logger         log.Logger
	PluginSettings *models.PluginSettings
	AuthSettings   *AuthSettings
}

type AuthSettings struct {
	AccessKey   string
	BearerToken string
}

func LogicmonitorBackendDataSource(dsSettings backend.DataSourceInstanceSettings) (instancemgmt.Instance, error) {
	logger := log.New()
	logger.Debug("Initializing new data source instance")

	var pluginSettings models.PluginSettings
	err := json.Unmarshal(dsSettings.JSONData, &pluginSettings)
	if err != nil {
		logger.Error("Error unmarshalling the Plugin Settings", err)
		return nil, err
	}
	return &LogicmonitorDataSource{
		dsInfo:         &dsSettings,
		Logger:         logger,
		PluginSettings: &pluginSettings,
		AuthSettings: &AuthSettings{
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

func (ds *LogicmonitorDataSource) QueryData(ctx context.Context, req *backend.QueryDataRequest) (*backend.QueryDataResponse, error) {
	// create response struct
	response := backend.NewQueryDataResponse()

	// loop over queries and execute them individually.
	for _, q := range req.Queries {
		res := logicmonitor.Query(ctx, ds, req.PluginContext, q)

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
func (ds *LogicmonitorDataSource) CheckHealth(_ context.Context, req *backend.CheckHealthRequest) (*backend.CheckHealthResult, error) {

	var status = backend.HealthStatusError
	var message = "Datasource Health Check Failed"

	status, message, result, err2, done := ds.validatePluginSettings(status, message)
	if done {
		return result, err2
	}

	resp, err := httpClient.Get(ds.PluginSettings.AccessID, ds.AuthSettings.AccessKey, ds.AuthSettings.BearerToken,
		constants.DeviceDevicesPath, constants.DevicesSizeOnePath, ds.PluginSettings.Path, ds.PluginSettings.Version)
	if err != nil {
		return &backend.CheckHealthResult{
			Status:  backend.HealthStatusError,
			Message: "Invalid Company name",
		}, nil
	}

	if resp.StatusCode == 503 || resp.StatusCode == 500 || resp.StatusCode == 400 {
		status = backend.HealthStatusError
		message = "Host not reachable / invalid company name configured"
		return &backend.CheckHealthResult{
			Status:  status,
			Message: message,
		}, nil
	}
	bodyText, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		ds.Logger.Info("Error Unmarshalling healthcheck  => ", err.Error)
	}
	deviceData := models.DeviceData{}
	err = json.Unmarshal(bodyText, &deviceData)
	if err != nil {
		return nil, err
	}
	if deviceData.Status == 200 {
		status = backend.HealthStatusOk
		message = "Authentication Success"
	} else if deviceData.Status == 1401 {
		status = backend.HealthStatusError
		message = "" + deviceData.Errmsg
	} else if deviceData.Status == 400 {
		status = backend.HealthStatusError
		message = "Invalid Token for Company or " + deviceData.Errmsg
	} else {
		status = backend.HealthStatusError
		message = "" + deviceData.Errmsg
	}

	return &backend.CheckHealthResult{
		Status:  status,
		Message: message,
	}, nil
}

func (ds *LogicmonitorDataSource) validatePluginSettings(status backend.HealthStatus, message string) (backend.HealthStatus, string, *backend.CheckHealthResult, error, bool) {
	if ds.PluginSettings.Path == "" {
		status = backend.HealthStatusError
		message = "Company name not entered"
		return 0, "", &backend.CheckHealthResult{
			Status:  status,
			Message: message,
		}, nil, true
	}

	if !ds.PluginSettings.IsLMV1Enabled && !ds.PluginSettings.IsBearerEnabled {
		return 0, "", &backend.CheckHealthResult{
			Status:  backend.HealthStatusError,
			Message: "Please Authenticate to use the plugin",
		}, nil, true
	}

	if !ds.PluginSettings.IsBearerEnabled {
		ds.AuthSettings.BearerToken = ""
	} else {
		if ds.AuthSettings.BearerToken == "" {
			return 0, "", &backend.CheckHealthResult{
				Status:  backend.HealthStatusError,
				Message: "Please enter bearer token",
			}, nil, true
		}
	}

	if ds.PluginSettings.IsLMV1Enabled {
		if ds.PluginSettings.AccessID == "" || ds.AuthSettings.AccessKey == "" {
			status = backend.HealthStatusError
			if ds.PluginSettings.AccessID == "" && ds.AuthSettings.AccessKey == "" {
				message = "Enable Lmv1 authentication methods and try again"
			}
			if ds.AuthSettings.AccessKey == "" {
				message = "Please enter Access Key"
			}
			if ds.PluginSettings.AccessID == "" {
				message = "Please enter AccessId"
			}
			return 0, "", &backend.CheckHealthResult{
				Status:  status,
				Message: message,
			}, nil, true
		}
	}
	return status, message, nil, nil, false
}

func (ds *LogicmonitorDataSource) CallResource(ctx context.Context, req *backend.CallResourceRequest, sender backend.CallResourceResponseSender) error {
	response := backend.DataResponse{}
	resp, err := httpClient.Get(ds.PluginSettings.AccessID, ds.AuthSettings.AccessKey, ds.AuthSettings.BearerToken,
		req.Path, req.URL, ds.PluginSettings.Path, ds.PluginSettings.Version)
	if err != nil {
		ds.Logger.Info(" Error from server => ", err)
		return response.Error
	}
	bodyText, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		ds.Logger.Info(" Error reading response => ", resp.Body)
	}
	return sender.Send(&backend.CallResourceResponse{
		Status: resp.StatusCode,
		Body:   []byte(bodyText),
	})
}

// SubscribeStream is called when a client wants to connect to a stream. This callback
// allows sending the first message.
//func (ds *LogicmonitorDataSource) SubscribeStream(_ context.Context, req *backend.SubscribeStreamRequest) (*backend.SubscribeStreamResponse, error) {
//	ds.Logger.Info("SubscribeStream called", "request", req)
//
//	status := backend.SubscribeStreamStatusPermissionDenied
//	if req.Path == "stream" {
//		// Allow subscribing only on expected path.
//		status = backend.SubscribeStreamStatusOK
//	}
//	return &backend.SubscribeStreamResponse{
//		Status: status,
//	}, nil
//}

// RunStream is called once for any open channel.  Results are shared with everyone
// subscribed to the same channel.
//func (ds *LogicmonitorDataSource) RunStream(ctx context.Context, req *backend.RunStreamRequest, sender *backend.StreamSender) error {
//	ds.Logger.Info("RunStream called", "request", req)
//
//	// Create the same data frame as for query data.
//	frame := data.NewFrame("response")
//
//	// Add fields (matching the same schema used in QueryData).
//	frame.Fields = append(frame.Fields,
//		data.NewField("time", nil, make([]time.Time, 1)),
//		data.NewField("values", nil, make([]int64, 1)),
//	)
//
//	counter := 0
//
//	// Stream data frames periodically till stream closed by Grafana.
//	for {
//		select {
//		case <-ctx.Done():
//			ds.Logger.Info("Context done, finish streaming", "path", req.Path)
//			return nil
//		case <-time.After(time.Second):
//			// Send new data periodically.
//			frame.Fields[0].Set(0, time.Now())
//			frame.Fields[1].Set(0, int64(10*(counter%2+1)))
//
//			counter++
//
//			err := sender.SendFrame(frame, data.IncludeAll)
//			if err != nil {
//				ds.Logger.Error("Error sending frame", "error", err)
//				continue
//			}
//		}
//	}
//}
//
//// PublishStream is called when a client sends a message to the stream.
//func (ds *LogicmonitorDataSource) PublishStream(_ context.Context, req *backend.PublishStreamRequest) (*backend.PublishStreamResponse, error) {
//	ds.Logger.Info("PublishStream called", "request", req)
//
//	// Do not allow publishing at all.
//	return &backend.PublishStreamResponse{
//		Status: backend.PublishStreamStatusPermissionDenied,
//	}, nil
//}
