package logicmonitor

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"time"

	"github.com/grafana/grafana-logicmonitor-datasource-backend/pkg/cache"
	"github.com/grafana/grafana-logicmonitor-datasource-backend/pkg/constants"
	"github.com/grafana/grafana-logicmonitor-datasource-backend/pkg/httpclient"
	"github.com/grafana/grafana-logicmonitor-datasource-backend/pkg/models"
	utils "github.com/grafana/grafana-logicmonitor-datasource-backend/pkg/utils"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/grafana/grafana-plugin-sdk-go/data"
)

/*
Start Task. This task is executed

 1. Only once when query request is for fixed time range set. Only one time because results are not going to be changed for fixed time range.
    Results from cache will be returned in subsequent calls

 2. When query request is for last X time and datasource collect interval is over from last time when data is recieved

 3. When Query is updated and datasource collect interval is over from last time when data is recieved

    So very first time this task is executed. in subsequent calls it waits for datasource collect interval from last timestamp data recieved
*/

type Job struct {
	JobId    int
	TimeFrom int64
	TimeTo   int64
}

func GetData(query backend.DataQuery, queryModel models.QueryModel, metaData models.MetaData, santabaClient httpclient.SantabaClient,
	pluginContext backend.PluginContext) backend.DataResponse {

	response := backend.DataResponse{}
	finalData := make(map[int]*models.MultiInstanceRawData)

	var prependTimeRangeForApiCall []models.PendingTimeRange
	var appendTimeRangeForApiCall []models.PendingTimeRange

	/*
		Step 1
		1. wait time is over/requets then calculate time range. Expect a new data if timeRangeForApiCall has entry
		2. Caclulate time range for rate limits records, multiple call will be made to each time range
	*/
	_, entryPresentInCache := cache.GetData(metaData)
	if queryModel.EnableStrategicApiCallFeature || !entryPresentInCache {
		response, prependTimeRangeForApiCall, appendTimeRangeForApiCall, metaData = cache.GetTimeRanges(query, queryModel, metaData, pluginContext,
			response, santabaClient.Logger)
	}

	// Validate with Single call first for any Errors
	finalData, response, queryModel = validateWithFirstCall(finalData, queryModel, metaData, santabaClient, pluginContext,
		response, prependTimeRangeForApiCall, appendTimeRangeForApiCall, false, santabaClient.Logger)
	if response.Error != nil {
		return response
	}

	/*
		Get earlier data than what is already in the cache
	*/
	finalData = initApiCallsAndAccomulateResponse(prependTimeRangeForApiCall, finalData, queryModel, metaData, santabaClient)
	santabaClient.Logger.Debug("Prepend Nr Of Entries", getNrOfEntries(finalData, 0))
	/*
		Get data from cache
	*/
	var cachedData *models.MultiInstanceRawData
	if data, ok := cache.GetData(metaData); ok {
		if cachedData, ok = data.(*models.MultiInstanceRawData); ok {
			finalData[len(finalData)] = cachedData
		}
		santabaClient.Logger.Debug("Cached Number of entries", getNrForSingleEntry(cachedData.Data.Instances))
	}

	/*
		Get latest data. expected more data than in cache
	*/
	lenTillCached := len(finalData)
	finalData = initApiCallsAndAccomulateResponse(appendTimeRangeForApiCall, finalData, queryModel, metaData, santabaClient)
	santabaClient.Logger.Debug("Append Number of entries", getNrOfEntries(finalData, lenTillCached))
	santabaClient.Logger.Debug("Total Number of entries", getNrOfEntries(finalData, 0))
	if len(finalData) == 0 {
		if response.Error == nil {
			response.Error = errors.New(constants.NoDataFromLM)
		}
	} else {
		response = processFinalData(queryModel, metaData, query.TimeRange.From.Unix(), query.TimeRange.To.Unix(), finalData, response, santabaClient.Logger)
		santabaClient.Logger.Debug("size of data in bytes", cache.GetRealSize(metaData))
	}

	return response
}

func validateWithFirstCall(finalData map[int]*models.MultiInstanceRawData, queryModel models.QueryModel, metaData models.MetaData,
	santabaClient httpclient.SantabaClient, pluginContext backend.PluginContext,
	response backend.DataResponse, prependTimeRangeForApiCall []models.PendingTimeRange, appendTimeRangeForApiCall []models.PendingTimeRange,
	seondCall bool, logger log.Logger) (map[int]*models.MultiInstanceRawData, backend.DataResponse, models.QueryModel) {
	if len(prependTimeRangeForApiCall) > 0 {
		finalData[0] = call(0, prependTimeRangeForApiCall[0].From, prependTimeRangeForApiCall[0].To, santabaClient, &queryModel, metaData)
	} else if len(appendTimeRangeForApiCall) > 0 {
		finalData[0] = call(0, appendTimeRangeForApiCall[0].From, appendTimeRangeForApiCall[0].To, santabaClient, &queryModel, metaData)
	}
	if len(finalData) > 0 && finalData[0].Error != "" && finalData[0].Error != "OK" {
		deviceMatched, _ := regexp.MatchString("Device<(.*?)> is not found", finalData[0].Error)
		deviceDataSourceMatched, _ := regexp.MatchString("DeviceDataSource<(.*?)> is not found", finalData[0].Error)
		if (deviceMatched || deviceDataSourceMatched) && !seondCall {
			queryModel, response = cache.InterpolateHostDetails(santabaClient, queryModel, response)
			queryModel, response = cache.InterpolateHostDataSourceDetails(santabaClient, queryModel, response)
			validateWithFirstCall(finalData, queryModel, metaData, santabaClient, pluginContext,
				response, prependTimeRangeForApiCall, appendTimeRangeForApiCall, true, logger)

		} else {
			response.Error = fmt.Errorf(finalData[0].Error)
			return finalData, response, queryModel
		}
	}
	return finalData, response, queryModel
}

/*
Initiate goroutines to call API for each time range caclulated
*/
func initApiCallsAndAccomulateResponse(timeRangeForApiCall []models.PendingTimeRange, rawDataMap map[int]*models.MultiInstanceRawData,
	queryModel models.QueryModel, metaData models.MetaData, santabaClient httpclient.SantabaClient) map[int]*models.MultiInstanceRawData {
	if len(timeRangeForApiCall) > 0 {
		dataLenIdx := len(rawDataMap)
		jobs := make(chan Job, len(timeRangeForApiCall)-1)
		results := make(chan *models.MultiInstanceRawData, len(timeRangeForApiCall)-1)
		for i := 1; i < len(timeRangeForApiCall); i++ {
			go callDataAPI(jobs, results, &queryModel, santabaClient, metaData)
		}
		for i := 1; i < len(timeRangeForApiCall); i++ {
			jobs <- Job{JobId: dataLenIdx, TimeFrom: timeRangeForApiCall[i].From, TimeTo: timeRangeForApiCall[i].To}
			dataLenIdx++
		}
		close(jobs)
		for i := 1; i < len(timeRangeForApiCall); i++ {
			result := <-results
			rawDataMap[result.JobId] = result
		}
		close(results)
	}
	return rawDataMap
}

// Gets fresh data by calling rest API
func callDataAPI(jobs chan Job, results chan<- *models.MultiInstanceRawData, queryModel *models.QueryModel, santabaClient httpclient.SantabaClient,
	metaData models.MetaData) {
	for job := range jobs {
		results <- call(job.JobId, job.TimeFrom, job.TimeTo, santabaClient, queryModel, metaData)
	}
}

func call(jobId int, fromTime int64, toTime int64, santabaClient httpclient.SantabaClient,
	queryModel *models.QueryModel, metaData models.MetaData) *models.MultiInstanceRawData {
	var rawData models.MultiInstanceRawData
	rawData.JobId = jobId
	rawData.FromTime = fromTime
	rawData.ToTime = toTime
	fullPath := utils.BuildURLReplacingQueryParams(constants.RawDataMultiInstanceReq, queryModel, rawData.FromTime, rawData.ToTime, metaData)
	santabaClient.Logger.Info("Calling API  => ", santabaClient.PluginSettings.Path, fullPath)
	respByte, err := santabaClient.Get(fullPath, constants.RawDataMultiInstanceReq)
	santabaClient.Logger.Info("Calling API Done  => ", santabaClient.PluginSettings.Path, fullPath)
	if err != nil {
		rawData.Error = err.Error()
		santabaClient.Logger.Error("Error from server => ", err)
	} else {
		err = json.Unmarshal(respByte, &rawData)
		if err != nil {
			rawData.Error = err.Error()
			santabaClient.Logger.Error(constants.ErrorUnmarshallingErrorData+"raw-data => ", err)
		}
	}
	return &rawData
}

// TODO currently only instanceData is filtered and stored in cache. to optimize cache usage, we can apply datapoint filter as well in case query is not edited
// TODO delete old data as per ttl
func processFinalData(queryModel models.QueryModel, metaData models.MetaData, from int64, to int64, rawDataMap map[int]*models.MultiInstanceRawData,
	response backend.DataResponse, logger log.Logger) backend.DataResponse {
	var dataFrameMap = make(map[string]*data.Frame)
	finalDataMerged := make(map[string]models.ValuesAndTime)
	// Below loop gets the recent data first. So as to reduce the cost of sorting
	for k := len(rawDataMap) - 1; k >= 0; k-- {
		if rawDataMap[k].Error != "OK" {
			response.Error = errors.New(rawDataMap[k].Error)
			break
		}
		cache.StoreFirstTimeStamp(metaData, rawDataMap[k].FromTime)
		for instanceName, valueAndTime := range rawDataMap[k].Data.Instances {
			// Check if instance selected/regex matching
			shortenInstance, matched := utils.IsInstanceMatched(metaData, &queryModel, rawDataMap[k].Data.DataSourceName, instanceName)
			if matched {
				if len(valueAndTime.Time) > 0 {
					cache.StoreLastTimeStamp(metaData, time.UnixMilli(valueAndTime.Time[0]).Unix())
				}
				metaData.MatchedInstances = true
				var frame *data.Frame
				dataPontMap := make(map[string]int)
				frame = utils.GetFrame(dataFrameMap, shortenInstance, queryModel.DataPointSelected)
				// this dataPontMap is to keep indexs of datapoints so as to get value from Values array for selected datapoints
				for i, v := range rawDataMap[k].Data.DataPoints {
					dataPontMap[v] = i
				}
				// filter only for time range from query And store in frame
				for i := 0; i < len(valueAndTime.Time); i++ {
					t := time.UnixMilli(valueAndTime.Time[i]).Unix()
					// Below check is to filter on time range
					if from <= t && to >= t {
						vals := make([]interface{}, len(frame.Fields))
						var idx = 1
						vals[0] = time.UnixMilli(valueAndTime.Time[i])
						for _, dp := range queryModel.DataPointSelected {
							fieldIdx := dataPontMap[dp.Label]
							if valueAndTime.Values[i][fieldIdx] == constants.NoData {
								vals[idx] = math.NaN()
							} else {
								vals[idx] = valueAndTime.Values[i][fieldIdx]
							}
							idx++
						}
						frame.AppendRow(vals...)
					} else if len(finalDataMerged) > 0 {
						// Time range is surpassed. just end the loop
						break
					}
				}
				dataFrameMap[shortenInstance] = frame
			}
			// Set Data if from QueryEditor. Donot store instance that is not selected/matching with regext, unless its for queryEditor
			if metaData.EditMode || matched {
				if _, ok := finalDataMerged[instanceName]; ok {
					finalDataMerged[instanceName] = models.ValuesAndTime{
						Time:   append(finalDataMerged[instanceName].Time, valueAndTime.Time...),
						Values: append(finalDataMerged[instanceName].Values, valueAndTime.Values...)}
				} else if len(valueAndTime.Time) > 0 && len(valueAndTime.Values) > 0 {
					finalDataMerged[instanceName] = models.ValuesAndTime{
						Time:   valueAndTime.Time,
						Values: valueAndTime.Values}
				}
			}
		}
	}
	// Check for errors, add frames to response and store data in cache
	if !metaData.MatchedInstances && len(dataFrameMap) == 0 && response.Error == nil {
		response.Error = errors.New(constants.InstancesNotMatchingWithHosts)
	} else {
		if len(dataFrameMap) > 0 {
			response.Frames = nil
			for _, frame := range dataFrameMap {
				response.Frames = append(response.Frames, frame)
			}
		}
		cache.StoreData(metaData, &models.MultiInstanceRawData{Data: models.MultiInstanceData{
			DataSourceName: rawDataMap[len(rawDataMap)-1].Data.DataSourceName,
			DataPoints:     rawDataMap[len(rawDataMap)-1].Data.DataPoints,
			Instances:      finalDataMerged},
			Error: "OK"})
	}
	return response
}

func getNrOfEntries(data map[int]*models.MultiInstanceRawData, from int) int {
	tot := 0
	if len(data) > 0 {
		for i := from; i < len(data); i++ {
			tot = tot + getNrForSingleEntry(data[i].Data.Instances)
		}
	}
	return tot
}

func getNrForSingleEntry(instances map[string]models.ValuesAndTime) int {
	var minNumOfEntriesFromInstances int = 0
	for _, newTimeValue := range instances {
		if minNumOfEntriesFromInstances < len(newTimeValue.Time) {
			minNumOfEntriesFromInstances = len(newTimeValue.Time)
		}
	}
	return minNumOfEntriesFromInstances
}
