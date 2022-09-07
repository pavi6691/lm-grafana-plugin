package plugin

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	b64 "encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"time"

	"github.com/ReneKroon/ttlcache"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/instancemgmt"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/grafana/grafana-plugin-sdk-go/data"
)

// Make sure SampleDatasource implements required interfaces. This is important to do
// since otherwise we will only get a not implemented error response from plugin in
// runtime. In this example datasource instance implements backend.QueryDataHandler,
// backend.CheckHealthHandler, backend.StreamHandler interfaces. Plugin should not
// implement all these interfaces - only those which are required for a particular task.
// For example if plugin does not need streaming functionality then you are free to remove
// methods that implement backend.StreamHandler. Implementing instancemgmt.InstanceDisposer
// is useful to clean up resources used by previous datasource instance when a new datasource
// instance created upon datasource settings changed.
var (
	_ backend.QueryDataHandler      = (*SampleDatasource)(nil)
	_ backend.CheckHealthHandler    = (*SampleDatasource)(nil)
	_ backend.StreamHandler         = (*SampleDatasource)(nil)
	_ instancemgmt.InstanceDisposer = (*SampleDatasource)(nil)
)

// NewSampleDatasource creates a new datasource instance.
func NewSampleDatasource(_ backend.DataSourceInstanceSettings) (instancemgmt.Instance, error) {
	return &SampleDatasource{}, nil
}

// SampleDatasource is an example datasource which can respond to data queries, reports
// its health and has streaming skills.
type SampleDatasource struct{}

// Dispose here tells plugin SDK that plugin wants to clean up resources when a new instance
// created. As soon as datasource settings change detected by SDK old datasource instance will
// be disposed and a new one will be created using NewSampleDatasource factory function.
func (d *SampleDatasource) Dispose() {
	// Clean up datasource instance resources.
}

// QueryData handles multiple queries and returns multiple responses.
// req contains the queries []DataQuery (where each query contains RefID as a unique identifier).
// The QueryDataResponse contains a map of RefID to the response for each query, and each response
// contains Frames ([]*Frame).

var cacheData = ttlcache.NewCache()

var lastExecutedTime = ttlcache.NewCache()

func (d *SampleDatasource) QueryData(ctx context.Context, req *backend.QueryDataRequest) (*backend.QueryDataResponse, error) {

	// create response struct
	response := backend.NewQueryDataResponse()

	// loop over queries and execute them individually.
	for _, q := range req.Queries {
		res := d.query(ctx, req.PluginContext, q)

		// save the response in a hashmap
		// based on with RefID as identifier
		response.Responses[q.RefID] = res
	}

	return response, nil
}

type Host struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

type Instance struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

type DataPoint struct {
	Label string `json:"label"`
	Value int64  `json:"value"`
}

type DataSource struct {
	Ds    int64  `json:"ds"`
	Label string `json:"label"`
	Value int64  `json:"value"`
}

type Data struct {
	DataSourceName string      `json:"dataSourceName"`
	DataPoints     []string    `json:"dataPoints"`
	Values         [][]float64 `json:"values"`
	Time           []int64     `json:"time"`
}

type RawData struct {
	Data Data `json:"data"`
}

type queryModel struct {
	HostSelected       Host        `json:"hostSelected"`
	HdsSelected        int64       `json:"hdsSelected"`
	DataSourceSelected DataSource  `json:"dataSourceSelected"`
	InstanceSelected   Instance    `json:"instanceSelected"`
	DataPointSelected  []DataPoint `json:"dataPointSelected"`
	WithStreaming      bool        `json:"withStreaming"`
	CollectInterval    int64       `json:"collectInterval"`
	UniqueId           string      `json:"uniqueId"`
}

func (d *SampleDatasource) query(c context.Context, pCtx backend.PluginContext, query backend.DataQuery) backend.DataResponse {
	response := backend.DataResponse{}
	// Unmarshal the JSON into our queryModel.
	var qm queryModel
	response.Error = json.Unmarshal([]byte(query.JSON), &qm)
	if response.Error != nil || qm.DataPointSelected == nil {
		return response
	}

	value, present := lastExecutedTime.Get(qm.UniqueId)
	if present && (value.(int64)+(qm.CollectInterval*1000)) > time.Now().UnixMilli() {
		frameValue, framePresent := cacheData.Get(qm.UniqueId)
		if framePresent {
			response.Frames = append(response.Frames, frameValue.(*data.Frame))
		} else {
			log.DefaultLogger.Error("Entry not exist in cache  => ", qm.UniqueId)
		}
		return response
	}

	var jsond JSONData
	AccessKey := pCtx.DataSourceInstanceSettings.DecryptedSecureJSONData["accessKey"]
	Bearer_token := pCtx.DataSourceInstanceSettings.DecryptedSecureJSONData["bearer_token"]
	response.Error = json.Unmarshal(pCtx.DataSourceInstanceSettings.JSONData, &jsond)
	if response.Error != nil {
		log.DefaultLogger.Info("response.Error", response.Error)
		return response
	}
	if !jsond.IsBearerEnabled {
		Bearer_token = ""
	}

	var fullPath string = "device/devices/" + qm.HostSelected.Value + fmt.Sprintf("%s%d", "/devicedatasources/", qm.HdsSelected) + "/instances/" + qm.InstanceSelected.Value + "/data" + fmt.Sprintf("%s%d", "?start=", query.TimeRange.From.Unix()) + fmt.Sprintf("%s%d", "&end=", query.TimeRange.To.Unix())
	var resourcePath string = "/device/devices/" + qm.HostSelected.Value + fmt.Sprintf("%s%d", "/devicedatasources/", qm.HdsSelected) + "/instances/" + qm.InstanceSelected.Value + "/data"

	log.DefaultLogger.Info("Calling API for query = ", qm)
	log.DefaultLogger.Info("Cache size = ", cacheData.Count())

	resp := call(jsond.AccessId, AccessKey, Bearer_token, resourcePath, fullPath, jsond.Path)
	bodyText, err := ioutil.ReadAll(resp.Body)
	if err != nil || resp.StatusCode != 200 {
		log.DefaultLogger.Info(" Error reading responce => ", resp.Body)
		return response
	}

	rawdata := RawData{}
	response.Error = json.Unmarshal(bodyText, &rawdata)
	if response.Error != nil {
		log.DefaultLogger.Info("Error Unmarshaling rawdata => ", response.Error)
		return response
	}
	frame := buildFrame(qm.DataPointSelected, rawdata.Data)
	// add the frames to the response.
	response.Frames = append(response.Frames, frame)

	cacheData.SetWithTTL(qm.UniqueId, frame, time.Duration(time.Duration(qm.CollectInterval+10)*time.Second))
	lastExecutedTime.SetWithTTL(qm.UniqueId, time.Now().UnixMilli(), time.Duration(time.Duration(qm.CollectInterval+10)*time.Second))

	return response
}

func buildFrame(dataPointSelected []DataPoint, rawdata Data) *data.Frame {
	// create data frame response.
	frame := data.NewFrame("response")

	// add fields
	frame.Fields = append(frame.Fields,
		data.NewField("time", nil, []time.Time{}),
	)

	for _, element := range dataPointSelected {
		frame.Fields = append(frame.Fields,
			data.NewField(element.Label, nil, []float64{}),
		)
	}
	for i, values := range rawdata.Values {
		vals := make([]interface{}, len(frame.Fields))
		var idx int = 1
		vals[0] = time.UnixMilli(rawdata.Time[i])
		for j, dp := range rawdata.DataPoints {
			for _, field := range frame.Fields {
				if field.Name == dp {
					vals[idx] = values[j]
					idx++
					break
				}
			}
		}
		frame.AppendRow(vals...)
	}
	return frame
}

// CheckHealth handles health checks sent from Grafana to the plugin.
// The main use case for these health checks is the test button on the
// datasource configuration page which allows users to verify that
// a datasource is working as expected.
func (d *SampleDatasource) CheckHealth(_ context.Context, req *backend.CheckHealthRequest) (*backend.CheckHealthResult, error) {
	log.DefaultLogger.Info("CheckHealth called", "request", req)

	var status = backend.HealthStatusOk
	var message = "Data source is working"

	if rand.Int()%2 == 0 {
		status = backend.HealthStatusError
		message = "randomized error"
	}

	return &backend.CheckHealthResult{
		Status:  status,
		Message: message,
	}, nil
}

type JSONData struct {
	Path            string `json:"path"`
	AccessId        string `json:"accessId"`
	IsBearerEnabled bool   `json:"isBearerEnabled"`
}

func (d *SampleDatasource) CallResource(ctx context.Context, req *backend.CallResourceRequest, sender backend.CallResourceResponseSender) error {
	response := backend.DataResponse{}
	var jsond JSONData
	AccessKey := req.PluginContext.DataSourceInstanceSettings.DecryptedSecureJSONData["accessKey"]
	Bearer_token := req.PluginContext.DataSourceInstanceSettings.DecryptedSecureJSONData["bearer_token"]
	if !jsond.IsBearerEnabled {
		Bearer_token = ""
	}
	response.Error = json.Unmarshal(req.PluginContext.DataSourceInstanceSettings.JSONData, &jsond)
	if response.Error != nil {
		log.DefaultLogger.Info("response.Error", response.Error)
		return response.Error
	}
	resp := call(jsond.AccessId, AccessKey, Bearer_token, req.Path, req.URL, jsond.Path)
	bodyText, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.DefaultLogger.Info(" Error reading responce => ", resp.Body)
	}
	return sender.Send(&backend.CallResourceResponse{
		Status: resp.StatusCode,
		Body:   []byte(bodyText),
	})
}

func call(accessId, accessKey, Bearer_token, resourcePath, fullPath, host string) *http.Response {
	var url string = "https://" + host + ".logicmonitor.com/santaba/rest/"
	url = url + fullPath
	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.DefaultLogger.Info(" Error creating http request => ", err)
	}
	if len(Bearer_token) > 0 {
		req.Header.Add("Authorization", "Bearer "+Bearer_token)
	} else {
		req.Header.Add("Authorization", getLMv1(accessId, accessKey, "/"+resourcePath))
	}
	if resourcePath == "autocomplete/names" {
		req.Header.Add("x-version", "3")
	}
	resp, err := client.Do(req)
	if err != nil {
		log.DefaultLogger.Info(" Error executing => "+url, err)
	}
	return resp
}

func getLMv1(accessId, accessKey, resourcePath string) string {
	epoch := time.Now().UnixMilli()
	getEpoch := fmt.Sprintf("%s%d", "GET", epoch)
	data := getEpoch + resourcePath
	h := hmac.New(sha256.New, []byte(accessKey))
	h.Write([]byte(data))
	sha := hex.EncodeToString(h.Sum(nil))
	auth := "LMv1 " + accessId + ":" + b64.URLEncoding.EncodeToString([]byte(sha)) + fmt.Sprintf("%s%d", ":", epoch)
	return auth
}

// SubscribeStream is called when a client wants to connect to a stream. This callback
// allows sending the first message.
func (d *SampleDatasource) SubscribeStream(_ context.Context, req *backend.SubscribeStreamRequest) (*backend.SubscribeStreamResponse, error) {
	log.DefaultLogger.Info("SubscribeStream called", "request", req)

	status := backend.SubscribeStreamStatusPermissionDenied
	if req.Path == "stream" {
		// Allow subscribing only on expected path.
		status = backend.SubscribeStreamStatusOK
	}
	return &backend.SubscribeStreamResponse{
		Status: status,
	}, nil
}

// RunStream is called once for any open channel.  Results are shared with everyone
// subscribed to the same channel.
func (d *SampleDatasource) RunStream(ctx context.Context, req *backend.RunStreamRequest, sender *backend.StreamSender) error {
	log.DefaultLogger.Info("RunStream called", "request", req)

	// Create the same data frame as for query data.
	frame := data.NewFrame("response")

	// Add fields (matching the same schema used in QueryData).
	frame.Fields = append(frame.Fields,
		data.NewField("time", nil, make([]time.Time, 1)),
		data.NewField("values", nil, make([]int64, 1)),
	)

	counter := 0

	// Stream data frames periodically till stream closed by Grafana.
	for {
		select {
		case <-ctx.Done():
			log.DefaultLogger.Info("Context done, finish streaming", "path", req.Path)
			return nil
		case <-time.After(time.Second):
			// Send new data periodically.
			frame.Fields[0].Set(0, time.Now())
			frame.Fields[1].Set(0, int64(10*(counter%2+1)))

			counter++

			err := sender.SendFrame(frame, data.IncludeAll)
			if err != nil {
				log.DefaultLogger.Error("Error sending frame", "error", err)
				continue
			}
		}
	}
}

// PublishStream is called when a client sends a message to the stream.
func (d *SampleDatasource) PublishStream(_ context.Context, req *backend.PublishStreamRequest) (*backend.PublishStreamResponse, error) {
	log.DefaultLogger.Info("PublishStream called", "request", req)

	// Do not allow publishing at all.
	return &backend.PublishStreamResponse{
		Status: backend.PublishStreamStatusPermissionDenied,
	}, nil
}
