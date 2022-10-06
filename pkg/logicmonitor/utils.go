package logicmonitor

import (
	"fmt"
	"math"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/grafana/grafana-logicmonitor-datasource-backend/pkg/constants"
	. "github.com/grafana/grafana-logicmonitor-datasource-backend/pkg/constants"
	"github.com/grafana/grafana-logicmonitor-datasource-backend/pkg/models"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data"
)

// BuildFrameFromMultiInstance = build frames for response with multi instances ref @MultiInstanceDataUrl
func BuildFrameFromMultiInstance(queryModel *models.QueryModel, data *models.MultiInstanceData) backend.DataResponse {
	response := backend.DataResponse{} //nolint:exhaustivestruct
	if queryModel.ValidInstanceRegex && queryModel.InstanceSelectBy == constants.Regex {
		for key := range data.Instances {
			instace := key[strings.IndexByte(key, '-')+1:]
			match, err := regexp.MatchString(queryModel.InstanceRegex, instace)
			if err == nil && match {
				dataFrame := buildFrame(instace, queryModel.DataPointSelected, data.DataPoints, data.Instances[key].Values, data.Instances[key].Time) //nolint:lll
				// add the frames to the response.
				response.Frames = append(response.Frames, dataFrame)
			}
		}
	} else {
		for _, instance := range queryModel.InstanceSelected {
			key := instance.Label

			_, ok := data.Instances[data.DataSourceName+string(DataSourceAndInstanceDelim)+instance.Label]
			if ok {
				key = data.DataSourceName + string(DataSourceAndInstanceDelim) + instance.Label
			} else {
				_, ok = data.Instances[data.DataSourceName+instance.Label]
				if ok {
					key = data.DataSourceName + instance.Label
				}
			}
			dataFrame := buildFrame(instance.Label, queryModel.DataPointSelected, data.DataPoints, data.Instances[key].Values, data.Instances[key].Time) //nolint:lll
			// add the frames to the response.
			response.Frames = append(response.Frames, dataFrame)
		}
	}

	return response
}

// build frames for given datapoints, values and time
func buildFrame(instanceName string, dataPointSelected []models.LabelIntValue, dataPoints []string, Values [][]interface{}, Time []int64) *data.Frame {
	// create data frame response.
	frame := data.NewFrame(ResponseStr)

	// add fields
	frame.Fields = append(frame.Fields,
		data.NewField(TimeStr, nil, []time.Time{}),
	)

	for _, datapoint := range dataPointSelected {
		frame.Fields = append(frame.Fields,
			data.NewField(instanceName+InstantAndDpDelim+datapoint.Label, nil, []float64{}),
		)
	}

	// this dataPontMap is to keep indexs of datapoints as value,
	// so as to get relevant value from Values array for selected data points
	dataPontMap := make(map[string]int)
	for i, v := range dataPoints {
		dataPontMap[v] = i
	}

	for i, values := range Values {
		vals := make([]interface{}, len(frame.Fields))
		var idx = 1
		vals[0] = time.UnixMilli(Time[i])

		for _, dp := range dataPointSelected {
			fieldIdx := dataPontMap[dp.Label]
			if values[fieldIdx] == NoData {
				vals[idx] = math.NaN()
			} else {
				vals[idx] = values[fieldIdx]
			}
			idx++
		}
		frame.AppendRow(vals...)
	}

	return frame
}

//nolint:cyclop
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
			qm.InstanceSelected[0].Value, query.TimeRange.From.Unix(), query.TimeRange.To.Unix())
	case RawDataMultiInstanceReq:
		return fmt.Sprintf(RawDataMultiInstanceURL, qm.HostSelected.Value, qm.HdsSelected, query.TimeRange.From.Unix(),
			query.TimeRange.To.Unix())
	case AllHostReq:
		return AllHostURL
	case AllInstanceReq:
		return fmt.Sprintf(AllInstanceURL, qm.HostSelected.Value, qm.HdsSelected)
	default:
		return RequestNotValidStr
	}
}

func UnixTruncateToNearestMinute(inputTime time.Time, intervalMin int64) int64 {
	inputTimeTruncated := inputTime.Truncate(time.Duration(intervalMin) * time.Second)

	return inputTimeTruncated.Unix()
}
