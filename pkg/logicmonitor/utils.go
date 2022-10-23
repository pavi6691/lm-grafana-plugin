package logicmonitor

import (
	"fmt"
	"math"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/grafana/grafana-logicmonitor-datasource-backend/pkg/cache"
	"github.com/grafana/grafana-logicmonitor-datasource-backend/pkg/constants"
	. "github.com/grafana/grafana-logicmonitor-datasource-backend/pkg/constants"
	"github.com/grafana/grafana-logicmonitor-datasource-backend/pkg/models"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/grafana/grafana-plugin-sdk-go/data"
)

// BuildFrameFromMultiInstance = build frames for response with multi instances ref @MultiInstanceDataUrl
func BuildFrameFromMultiInstance(uniqueID string, queryModel *models.QueryModel, instanceData *models.MultiInstanceData, tempMap map[string]*data.Frame, logger log.Logger) (map[string]*data.Frame, bool) {
	var matchedInstances bool = false
	if queryModel.ValidInstanceRegex && queryModel.InstanceSelectBy == constants.Regex {
		for key := range instanceData.Instances {
			instace := key[strings.IndexByte(key, '-')+1:]
			match, err := regexp.MatchString(queryModel.InstanceRegex, instace)
			if err == nil && match {
				matchedInstances = true
				dataFrame := addFrameValues(uniqueID, instace, queryModel.DataPointSelected, instanceData.DataPoints, instanceData.Instances[key].Values, instanceData.Instances[key].Time, tempMap, logger) //nolint:lll
				// add the frames to the response.
				tempMap[instace] = dataFrame
			}
		}
	} else {
		for _, instance := range queryModel.InstanceSelected {
			key := instance.Label

			/* form key for the instance map, below is the instance name formation from santaba
			if (ds.getHasMultiInstance()) {
				if (ds.getName().endsWith("-")) {
			        builder.setName(ds.getName() + alias);
				} else {
			        builder.setName(ds.getName() + "-" + alias);
			    }
			} else {
			    builder.setName(ds.getName() + alias);
			}
			*/
			_, ok := instanceData.Instances[instanceData.DataSourceName+string(DataSourceAndInstanceDelim)+instance.Label]
			if ok {
				key = instanceData.DataSourceName + string(DataSourceAndInstanceDelim) + instance.Label
			} else {
				_, ok = instanceData.Instances[instanceData.DataSourceName+instance.Label]
				if ok {
					key = instanceData.DataSourceName + instance.Label
				}
			}
			_, ok = instanceData.Instances[instanceData.DataSourceName+instance.Label]
			if ok {
				matchedInstances = true
			}
			dataFrame := addFrameValues(uniqueID, instance.Label, queryModel.DataPointSelected, instanceData.DataPoints, instanceData.Instances[key].Values, instanceData.Instances[key].Time, tempMap, logger) //nolint:lll
			// add the frames to the response.
			tempMap[instance.Label] = dataFrame
		}
	}

	return tempMap, matchedInstances
}

func RecordsToAppend(query backend.DataQuery, collectInterval int64, uniqueId string) int64 {
	totalNumOfRecords := ((query.TimeRange.To.Unix() - query.TimeRange.From.Unix()) / 60) / (collectInterval / 60)
	return totalNumOfRecords - int64(cache.GetFrameCount(uniqueId))
}

func GetTimeRanges(query backend.DataQuery, collectInterval int64, uniqueId string) []models.PendingTimeRange {
	pendingTimeRange := []models.PendingTimeRange{}
	recordsToAppend := RecordsToAppend(query, collectInterval, uniqueId)
	lt := cache.GetLastTime(uniqueId)
	var from int64
	if lt > 0 {
		from = lt + 1
	} else {
		from = UnixTruncateToNearestMinute(query.TimeRange.From, 60)
	}
	for i := recordsToAppend; i > 500; i = i - 500 {
		to := from + (500 * collectInterval)
		if time.Now().Unix() < from+(500*collectInterval) {
			pendingTimeRange = append(pendingTimeRange, models.PendingTimeRange{From: from, To: time.Now().Unix()})
			recordsToAppend = 0
			break
		} else {
			pendingTimeRange = append(pendingTimeRange, models.PendingTimeRange{From: from, To: from + (500 * collectInterval)})
		}
		recordsToAppend = recordsToAppend - 500
		from = to + 1
	}
	if recordsToAppend > 0 {
		if time.Now().Unix() < from+(recordsToAppend*collectInterval) {
			pendingTimeRange = append(pendingTimeRange, models.PendingTimeRange{From: from, To: time.Now().Unix()})
		} else {
			pendingTimeRange = append(pendingTimeRange, models.PendingTimeRange{From: from, To: from + (recordsToAppend * collectInterval)})
		}
	}
	cache.StoreLastTime(uniqueId, UnixTruncateToNearestMinute(time.Now(), 60))
	return pendingTimeRange
}

func addFrameValues(uniqueID string, instanceName string, dataPointSelected []models.LabelIntValue, dataPoints []string, Values [][]interface{}, Time []int64, tempMap map[string]*data.Frame, logger log.Logger) *data.Frame {
	// this dataPontMap is to keep indexs of datapoints as value,
	// so as to get relevant value from Values array for selected data points
	frame := getFrame(tempMap, instanceName, dataPointSelected, logger)
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
	// if len(Time) > 0 && cache.GetLastTime(uniqueID) < Time[0] {
	// 	cache.StoreTimeStamWithNewEntry(uniqueID, Time[0])
	// }
	return frame
}

// build frames for given datapoints, values and time. get existing frame if its for the same instance, this is in case of multiple rawdata api calls
func getFrame(tempMap map[string]*data.Frame, instanceName string, dataPointSelected []models.LabelIntValue, logger log.Logger) *data.Frame {
	// create data frame response.
	val, ok := tempMap[instanceName]
	if ok {
		return val
	} else {
		frame := data.NewFrame(ResponseStr)
		// add fields
		frame.Fields = append(frame.Fields,
			data.NewField(TimeStr, nil, []time.Time{}),
		)
		frame.RefID = instanceName
		for _, datapoint := range dataPointSelected {
			frame.Fields = append(frame.Fields,
				data.NewField(instanceName+InstantAndDpDelim+datapoint.Label, nil, []float64{}),
			)
		}
		return frame
	}
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
	case HostDataSourceReq:
		return fmt.Sprintf(HostDataSourceURL, qm.HostSelected.Value, qm.DataSourceSelected.Ds)
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
