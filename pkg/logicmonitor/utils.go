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
	"github.com/grafana/grafana-logicmonitor-datasource-backend/pkg/models"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/grafana/grafana-plugin-sdk-go/data"
)

// build frames for response with multi instances ref @MultiInstanceDataUrl
func BuildFrameFromMultiInstance(uniqueID string, queryModel *models.QueryModel, instanceData *models.MultiInstanceData, tempMap map[string]*data.Frame,
	metadata models.MetaData, logger log.Logger) (map[string]*data.Frame, bool) {
	var matchedInstances bool = false
	if queryModel.EnableRegexFeature && queryModel.ValidInstanceRegex && queryModel.InstanceSelectBy == constants.Regex {
		for key := range instanceData.Instances {
			instace := key[strings.IndexByte(key, '-')+1:]
			match, err := regexp.MatchString(queryModel.InstanceRegex, instace)
			if err == nil && match {
				matchedInstances = true
				dataFrame := addFrameValues(uniqueID, instace, queryModel.DataPointSelected, instanceData.DataPoints, instanceData.Instances[key].Values,
					instanceData.Instances[key].Time, tempMap, queryModel, metadata, logger) //nolint:lll
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
			_, ok := instanceData.Instances[instanceData.DataSourceName+string(constants.DataSourceAndInstanceDelim)+instance.Label]
			if ok {
				key = instanceData.DataSourceName + string(constants.DataSourceAndInstanceDelim) + instance.Label
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
			dataFrame := addFrameValues(uniqueID, instance.Label, queryModel.DataPointSelected, instanceData.DataPoints,
				instanceData.Instances[key].Values, instanceData.Instances[key].Time, tempMap, queryModel, metadata, logger) //nolint:lll
			// add the frames to the response.
			tempMap[instance.Label] = dataFrame
		}
	}

	return tempMap, matchedInstances
}

func addFrameValues(uniqueID string, instanceName string, dataPointSelected []models.LabelIntValue, dataPoints []string, Values [][]interface{},
	Time []int64, tempMap map[string]*data.Frame, queryModel *models.QueryModel, metaData models.MetaData, logger log.Logger) *data.Frame {
	frame := getFrame(uniqueID, tempMap, instanceName, dataPointSelected, Values, metaData, logger)

	// this dataPontMap is to keep indexs of datapoints as value,
	// so as to get relevant value from Values array for selected data points
	dataPontMap := make(map[string]int)
	for i, v := range dataPoints {
		dataPontMap[v] = i
	}
	// first entry for recent timestamp, so append from last to first, so the sequence is maintained
	for i := len(Values) - 1; i >= 0; i-- {
		vals := make([]interface{}, len(frame.Fields))
		var idx = 1
		vals[0] = time.UnixMilli(Time[i])
		for _, dp := range dataPointSelected {
			fieldIdx := dataPontMap[dp.Label]
			if Values[i][fieldIdx] == constants.NoData {
				vals[idx] = math.NaN()
			} else {
				vals[idx] = Values[i][fieldIdx]
			}
			idx++
		}
		frame.AppendRow(vals...)
	}
	if queryModel.EnableDataAppendFeature && len(Time) > 0 {
		latestTimeOfAllInstances := time.UnixMilli(Time[0]).Unix()
		if cache.GetLastestRawDataEntryTimestamp(metaData, queryModel.EnableDataAppendFeature) < latestTimeOfAllInstances {
			cache.StoreLastestRawDataEntryTimestamp(metaData, latestTimeOfAllInstances, metaData.FrameCacheTTLInSeconds)
		}
	}
	return frame
}

/*
Gets frames for given datapoints, values and time.
Get frame from cache if present. this is the case when data will be just appended
If not present in the cache then its the first call.
Get existing frame if its for the same instance. This is in case of multiple rawdata api calls
*/
func getFrame(uniqueID string, tempMap map[string]*data.Frame, instanceName string, dataPointSelected []models.LabelIntValue, Values [][]interface{},
	metaData models.MetaData, logger log.Logger) *data.Frame {

	// check if data frame for this instance is already present
	var frame *data.Frame

	/*
		Get frame from cache in case its "not" from queryEditorCache, if from queryEditorCache tempMap has already old as well as new data
	*/
	if !metaData.IsCallFromQueryEditor {
		frameValue, framePresent := cache.GetData(uniqueID)
		if framePresent {
			if df, ok := frameValue.(backend.DataResponse); ok {
				for _, frame = range df.Frames {
					if frame.RefID == instanceName {
						break
					}
				}
			}
		}
	}
	/*
		frame is not from cache, this is the first call
	*/
	if frame == nil {
		// create data frame response.
		val, ok := tempMap[instanceName]
		if ok {
			if metaData.IsCallFromQueryEditor && metaData.AppendAndDelete && !metaData.AppendOnly {
				if len(Values) < val.Rows() {
					// here, if its for fromQueryEditor and is to append data
					// delete same number of intial entries as new entries to append
					logger.Debug("Re-arrangig dataFrame : Nr of enties deleted and same number of new entries are appended.....", len(Values))
					for i := 0; i < len(Values); i++ {
						val.DeleteRow(i)
					}
				} else {
					return initiateNewDataFrame(instanceName, dataPointSelected)
				}
			}
			return val
		} else {
			return initiateNewDataFrame(instanceName, dataPointSelected)
		}
	} else if !metaData.AppendOnly {
		/*
			On dashboard/query is not updated recently. data already present and its a append request. dataframe is recent,
			so to append new data, same number of initial records are removed
		*/
		if len(Values) < frame.Rows() {
			/*
				TODO importent! When new entries are appended, same amount of old entries are removed, need to check all those new entries are for
				all datapoints selected, for missing dps also old entries are removed which is not correct. it may happen in case of multiple instance,
				not all instance will get entries at the same time
			*/
			logger.Debug("Re-arrangig dataFrame : Nr of enties deleted and same number of new entries are appended.....", len(Values))
			for i := 0; i < len(Values); i++ {
				frame.DeleteRow(i)
			}
		} else {
			return initiateNewDataFrame(instanceName, dataPointSelected)
		}
	}
	return frame
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

func RecordsToAppend(from int64, to int64, collectInterval int64) int64 {
	totalNumOfRecords := ((to - from) / 60) / (collectInterval / 60)
	return totalNumOfRecords
}

func GetTimeRanges(from int64, to int64, collectInterval int64, metaData models.MetaData, logger log.Logger) []models.PendingTimeRange {
	pendingTimeRange := []models.PendingTimeRange{}
	recordsToAppend := RecordsToAppend(from, to, collectInterval)
	logger.Debug("RecordsToAppend => ", recordsToAppend)
	for i := recordsToAppend; i > constants.NumberOfRecordsWithRateLimit; i = i - constants.NumberOfRecordsWithRateLimit {
		to := from + (constants.NumberOfRecordsWithRateLimit * collectInterval)
		if time.Now().Unix() < to {
			pendingTimeRange = append(pendingTimeRange, models.PendingTimeRange{From: from, To: time.Now().Unix()})
			recordsToAppend = 0
			break
		} else {
			pendingTimeRange = append(pendingTimeRange, models.PendingTimeRange{From: from, To: to})
		}
		recordsToAppend = recordsToAppend - constants.NumberOfRecordsWithRateLimit
		from = to + 1
	}
	if recordsToAppend > 0 {
		if metaData.IsForLastXTime {
			pendingTimeRange = append(pendingTimeRange, models.PendingTimeRange{From: from, To: time.Now().Unix()})
		} else {
			pendingTimeRange = append(pendingTimeRange, models.PendingTimeRange{From: from, To: from + (recordsToAppend * collectInterval)})
		}
	}
	return pendingTimeRange
}

/*
Returns seconds to wait before making API call for new data. is based on ds collect interval time and last time when data is recieved
*/
func GetWaitTimeInSec(metaData models.MetaData, collectInterval int64, enableDataAppendFeature bool) int64 {
	waitSeconds := (cache.GetLastestRawDataEntryTimestamp(metaData, enableDataAppendFeature) + collectInterval) - time.Now().Unix()
	if waitSeconds > 0 {
		return waitSeconds
	}
	return 0
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
		if metaData.IsCallFromQueryEditor {
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
