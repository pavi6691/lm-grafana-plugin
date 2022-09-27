package logicmonitor

import (
	"fmt"
	"math"
	"net/url"
	"strings"
	"time"

	"github.com/grafana/grafana-logicmonitor-datasource-backend/pkg/constants"
	. "github.com/grafana/grafana-logicmonitor-datasource-backend/pkg/constants"
	"github.com/grafana/grafana-logicmonitor-datasource-backend/pkg/models"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data"
)

// build frames for response with multi instances ref @constants.MultiInstaceDataUrl
func buildFrameFromMultiInstace(qm models.QueryModel, data models.MultiInstanceData) backend.DataResponse {
	response := backend.DataResponse{}
	for _, instance := range qm.InstanceSelected {
		_, ok := data.Instances[data.DataSourceName+"-"+instance.Label]
		var key string
		if ok {
			key = data.DataSourceName + "-" + instance.Label
		} else {
			key = instance.Label
		}
		dataFrame := buildFrame(instance.Label, qm.DataPointSelected, data.DataPoints, data.Instances[key].Values, data.Instances[key].Time)
		// add the frames to the response.
		response.Frames = append(response.Frames, dataFrame)
	}
	return response
}

// build frames for given datapoints, values and time
func buildFrame(instanceName string, dataPointSelected []models.LabelIntValue, DataPoints []string, Values [][]interface{}, Time []int64) *data.Frame {
	// create data frame response.
	frame := data.NewFrame("response")

	// add fields
	frame.Fields = append(frame.Fields,
		data.NewField("time", nil, []time.Time{}),
	)

	for _, element := range dataPointSelected {
		frame.Fields = append(frame.Fields,
			data.NewField(instanceName+string(constants.InstantAndDpDelim)+element.Label, nil, []float64{}),
		)
	}
	for i, values := range Values {
		vals := make([]interface{}, len(frame.Fields))
		var idx int = 1
		vals[0] = time.UnixMilli(Time[i])
		for j, dp := range DataPoints {
			for _, field := range frame.Fields {
				if field.Name[strings.IndexByte(field.Name, constants.InstantAndDpDelim)+1:] == dp {
					if values[j] == "No Data" {
						vals[idx] = math.NaN()
					} else {
						vals[idx] = values[j]
					}
					idx++
					break
				}
			}
		}
		frame.AppendRow(vals...)
	}
	return frame
}

//todo
//func BuildRawDataPath(queryModel *models.QueryModel, query *backend.DataQuery) string {
//	return fmt.Sprintf(RawDataURL, queryModel.HostSelected.Value, queryModel.HdsSelected,
//		queryModel.InstanceSelected[0].Value, query.TimeRange.From.Unix(), query.TimeRange.To.Unix())
//}

//func BuildResourcePath(queryModel *models.QueryModel) string {
//	return fmt.Sprintf(constants.RawDataResourcePath, queryModel.HostSelected.Value, queryModel.HdsSelected,
//		queryModel.InstanceSelected.Value)
//}

func BuildURLReplacingQueryParams(request string, qm *models.QueryModel, query *backend.DataQuery) string {
	switch request {
	case AutoCompleteGroupReq:
		return fmt.Sprintf(AutoCompleteGroupURL, time.Now().UnixMilli(), url.QueryEscape(qm.GroupSelected.Label))
	case ServiceOrDeviceGroupReq:
		return fmt.Sprintf(ServiceOrDeviceGroupURL, time.Now().UnixMilli()) +
			url.QueryEscape(fmt.Sprintf(GroupExtraFilters, qm.TypeSelected, qm.GroupSelected.Label, qm.GroupSelected.Label))
	case AutoCompleteHostReq:
		return fmt.Sprintf(AutoCompleteHostsURL, time.Now().UnixMilli(), url.QueryEscape(qm.HostSelected.Label)) +
			url.QueryEscape(fmt.Sprintf(HostParentFilters, qm.GroupSelected.Label))
	case AutoCompleteInstanceReq:
		return fmt.Sprintf(AutoCompleteInstanceURL, time.Now().UnixMilli(), url.QueryEscape(qm.InstanceSearch)) +
			url.QueryEscape(fmt.Sprintf(InstanceParentFilters, qm.GroupSelected.Label,
				qm.HostSelected.Label, qm.DataSourceSelected.Label))
	case DataSourceReq:
		return fmt.Sprintf(DataSourceURL, qm.HostSelected.Value)
	case DataPointReq:
		return fmt.Sprintf(DataPointURL, qm.DataSourceSelected.Ds)
	case HealthCheckReq:
		return HealthCheckURL
	case RawDataSingleInstaceReq:
		return fmt.Sprintf(RawDataSingleInstanceURL, qm.HostSelected.Value, qm.HdsSelected,
			qm.InstanceSelected[0].Value, query.TimeRange.From.Unix(), query.TimeRange.To.Unix(), getDataPointNamesDelimByComma(qm.DataPointSelected))
	case RawDataMultiInstanceReq:
		return fmt.Sprintf(RawDataMultiInstanceURL, qm.HostSelected.Value, qm.HdsSelected, query.TimeRange.From.Unix(),
			query.TimeRange.To.Unix(), getDataPointNamesDelimByComma(qm.DataPointSelected))
	case AllHostReq:
		return AllHostURL
	case AllInstanceReq:
		return fmt.Sprintf(AllInstanceURL, qm.HostSelected.Value, qm.HdsSelected)
	default:
		return "Request not valid"
	}
}

func getDataPointNamesDelimByComma(lv []models.LabelIntValue) string {
	var s string
	for i, d := range lv {
		if i == 0 {
			s = d.Label
		} else {
			s = s + "," + d.Label
		}
	}
	return s
}
