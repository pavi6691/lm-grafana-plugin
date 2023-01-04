package logicmonitor

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/grafana/grafana-logicmonitor-datasource-backend/pkg/constants"
	"github.com/grafana/grafana-logicmonitor-datasource-backend/pkg/models"
	"github.com/grafana/grafana-plugin-sdk-go/data"
)

func IsInstanceMatched(metadata models.MetaData, queryModel *models.QueryModel, dataSourceName string, instanceName string) (string, bool) {
	if queryModel.EnableRegexFeature && queryModel.ValidInstanceRegex && queryModel.InstanceSelectBy == constants.Regex {
		instace := instanceName[strings.IndexByte(instanceName, '-')+1:]
		match, err := regexp.MatchString(queryModel.InstanceRegex, instace)
		return instace, err == nil && match
	} else if queryModel.InstanceSelectBy == constants.Select {
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
			if instanceName == dataSourceName+string(constants.DataSourceAndInstanceDelim)+instance.Label {
				key = dataSourceName + string(constants.DataSourceAndInstanceDelim) + instance.Label
			} else if instanceName == dataSourceName+instance.Label {
				key = dataSourceName + instance.Label
			}
			if instanceName == key {
				return instance.Label, true
			}
		}
	}
	return instanceName, false
}

/*
Gets frames for given datapoints, values and time.
Get frame from cache if present. this is the case when data will be just appended
If not present in the cache then its the first call.
Get existing frame if its for the same instance. This is in case of multiple rawdata api calls
*/
func getFrame(tempMap map[string]*data.Frame, instanceName string, dataPointSelected []models.LabelIntValue) *data.Frame {

	val, ok := tempMap[instanceName]
	if ok {
		return val
	} else {
		return initiateNewDataFrame(instanceName, dataPointSelected)
	}
}

func initiateNewDataFrame(instanceName string, dataPointSelected []models.LabelIntValue) *data.Frame {
	frame := data.NewFrame(constants.ResponseStr)
	// add fields
	frame.Fields = append(frame.Fields,
		data.NewField(constants.TimeStr, nil, []time.Time{}),
	)
	frame.RefID = instanceName
	for _, datapoint := range dataPointSelected {
		frame.Fields = append(frame.Fields,
			data.NewField(instanceName+constants.InstantAndDpDelim+datapoint.Label, nil, []float64{}),
		)
	}
	return frame
}

//nolint:cyclop
func BuildURLReplacingQueryParams(request string, qm *models.QueryModel, from int64, to int64, metaData models.MetaData) string {
	switch request {
	case constants.AutoCompleteGroupReq:
		return fmt.Sprintf(constants.AutoCompleteGroupURL, time.Now().UnixMilli(), url.QueryEscape(qm.GroupSelected.Label))
	case constants.ServiceOrDeviceGroupReq:
		return fmt.Sprintf(constants.ServiceOrDeviceGroupURL, time.Now().UnixMilli()) +
			url.QueryEscape(fmt.Sprintf(constants.GroupExtraFilters, qm.TypeSelected, qm.GroupSelected.Label, qm.GroupSelected.Label))
	case constants.AutoCompleteHostReq:
		return fmt.Sprintf(constants.AutoCompleteHostsURL, time.Now().UnixMilli(), url.QueryEscape(qm.HostSelected.Label)) +
			url.QueryEscape(fmt.Sprintf(constants.HostParentFilters, qm.GroupSelected.Label))
	case constants.AutoCompleteInstanceReq:
		return fmt.Sprintf(constants.AutoCompleteInstanceURL, time.Now().UnixMilli(), url.QueryEscape(qm.InstanceSearch)) +
			url.QueryEscape(fmt.Sprintf(constants.InstanceParentFilters, qm.GroupSelected.Label,
				qm.HostSelected.Label, qm.DataSourceSelected.Label))
	case constants.DataSourceReq:
		return fmt.Sprintf(constants.DataSourceURL, qm.HostSelected.Value)
	case constants.HostDataSourceReq:
		return fmt.Sprintf(constants.HostDataSourceURL, qm.HostSelected.Value, qm.DataSourceSelected.Ds)
	case constants.DataPointReq:
		return fmt.Sprintf(constants.DataPointURL, qm.DataSourceSelected.Ds)
	case constants.HealthCheckReq:
		return constants.HealthCheckURL
	case constants.RawDataSingleInstaceReq:
		return fmt.Sprintf(constants.RawDataSingleInstanceURL, qm.HostSelected.Value, qm.HdsSelected,
			qm.InstanceSelected[0].Value, from, to)
	case constants.RawDataMultiInstanceReq:
		if metaData.EditMode {
			return fmt.Sprintf(constants.RawDataMultiInstanceURL, qm.HostSelected.Value, qm.HdsSelected, from,
				to)
		} else {
			return fmt.Sprintf(constants.RawDataMultiInstanceURLWithDpFilter, qm.HostSelected.Value, qm.HdsSelected, from,
				to, getDps(qm.DataPointSelected))
		}
	case constants.AllHostReq:
		return constants.AllHostURL
	case constants.AllInstanceReq:
		return fmt.Sprintf(constants.AllInstanceURL, qm.HostSelected.Value, qm.HdsSelected)
	default:
		return constants.RequestNotValidStr
	}
}

func UnixTruncateToNearestMinute(inputTime int64, intervalMin int64) int64 {
	inputTimeTruncated := time.UnixMilli(inputTime * 1000).Truncate(time.Duration(intervalMin) * time.Second)

	return inputTimeTruncated.Unix()
}

func getDps(dataPointSelected []models.LabelIntValue) string {
	var tempDps string
	for i, d := range dataPointSelected {
		if i == len(dataPointSelected)-1 {
			tempDps = tempDps + d.Label
			break
		}
		tempDps = tempDps + d.Label + ","
	}
	return tempDps
}
