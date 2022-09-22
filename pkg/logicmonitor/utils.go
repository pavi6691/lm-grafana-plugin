package logicmonitor

import (
	"fmt"
	"github.com/grafana/grafana-logicmonitor-datasource-backend/pkg/constants"
	"github.com/grafana/grafana-logicmonitor-datasource-backend/pkg/models"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data"
	"math"
	"time"
)

func BuildFrame(dataPointSelected []models.DataPoint, rawdata models.Data) *data.Frame {
	// create data frame response.
	frame := data.NewFrame(constants.Response)

	frame.Fields = append(frame.Fields,
		data.NewField(constants.Time, nil, []time.Time{}),
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

					if values[j] == constants.NoData {
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

func BuildFullPath(queryModel *models.QueryModel, query *backend.DataQuery) string {
	return fmt.Sprintf(constants.RawDataFullPath, queryModel.HostSelected.Value, queryModel.HdsSelected, queryModel.InstanceSelected.Value, query.TimeRange.From.Unix(), query.TimeRange.To.Unix())
}

func BuildResourcePath(queryModel *models.QueryModel) string {
	return fmt.Sprintf(constants.RawDataResourcePath, queryModel.HostSelected.Value, queryModel.HdsSelected, queryModel.InstanceSelected.Value)
}
