package logicmonitor

import (
	"fmt"
	. "github.com/grafana/grafana-logicmonitor-datasource-backend/pkg/constants"
	"github.com/grafana/grafana-logicmonitor-datasource-backend/pkg/models"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data"
	"math"
	"net/url"
	"time"
)

func BuildFrame(dataPointSelected []models.LabelIntValue, rawData models.Data) *data.Frame {
	// create data frame response.
	frame := data.NewFrame(Response)

	frame.Fields = append(frame.Fields,
		data.NewField(Time, nil, []time.Time{}),
	)

	for _, element := range dataPointSelected {
		frame.Fields = append(frame.Fields,
			data.NewField(element.Label, nil, []float64{}),
		)
	}

	for i, values := range rawData.Values {
		vals := make([]interface{}, len(frame.Fields))

		var idx = 1
		vals[0] = time.UnixMilli(rawData.Time[i])

		for j, dp := range rawData.DataPoints {
			for _, field := range frame.Fields {
				if field.Name == dp {
					if values[j] == NoData {
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

func UnixTruncateToMinute(unixTime int64) int64 {
	t := time.Unix(unixTime, 0)
	return t.Truncate(time.Minute).Unix()
}

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
	case RawDataReq:
		return fmt.Sprintf(RawDataURL, qm.HostSelected.Value, qm.HdsSelected,
			qm.InstanceSelected[0].Value, query.TimeRange.From.Unix(), query.TimeRange.To.Unix())
	case AllHostReq:
		return AllHostURL
	case AllInstanceReq:
		return fmt.Sprintf(AllInstanceURL, qm.HostSelected.Value, qm.HdsSelected)
	default:

		return ""
	}
}
