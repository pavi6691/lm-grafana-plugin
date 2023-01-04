package logicmonitor

import (
	"encoding/json"
	"errors"
	"math"
	"time"

	"github.com/grafana/grafana-logicmonitor-datasource-backend/pkg/cache"
	"github.com/grafana/grafana-logicmonitor-datasource-backend/pkg/constants"
	"github.com/grafana/grafana-logicmonitor-datasource-backend/pkg/httpclient"
	"github.com/grafana/grafana-logicmonitor-datasource-backend/pkg/models"
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

func GetData(query backend.DataQuery, queryModel models.QueryModel, metaData models.MetaData, authSettings *models.AuthSettings,
	pluginSettings *models.PluginSettings, pluginContext backend.PluginContext, logger log.Logger) backend.DataResponse {

	response := backend.DataResponse{}
	finalData := make(map[int]*models.MultiInstanceRawData)

	//TODO remove start
	logger.Info("")
	logger.Info("ID", metaData.Id)
	logger.Info("Is in EditMode", metaData.EditMode)
	var prependTimeRangeForApiCall []models.PendingTimeRange
	var appendTimeRangeForApiCall []models.PendingTimeRange
	//TODO remove end

	/*
		Step 1
		1. wait time is over/requets then calculate time range. Expect a new data if timeRangeForApiCall has entry
		2. Caclulate time range for rate limits records, multiple call will be made to each time range
	*/
	_, entryPresentInCache := cache.GetData(metaData)
	if queryModel.EnableStrategicApiCallFeature || !entryPresentInCache {
		response, prependTimeRangeForApiCall, appendTimeRangeForApiCall, metaData = cache.GetTimeRanges(query, queryModel, metaData, pluginContext,
			response, logger)
	}
	// if response.Error != nil {
	// 	return response
	// }
	logger.Info("Prepend Nr Of Calls", len(prependTimeRangeForApiCall))
	logger.Info("Append Nr Of Calls", len(appendTimeRangeForApiCall))
	logger.Error("Current Api Calls", cache.GetNrOfApiCalls(pluginContext.DataSourceInstanceSettings.UID).NrOfCalls)
	/*
		Get earlier data than what is already in the cache
	*/
	finalData = initApiCallsAndAccomulateResponse(prependTimeRangeForApiCall, finalData, queryModel, metaData, authSettings, pluginSettings, logger)
	logger.Info("Prepend Nr Of Entries", getNrOfEntries(finalData, 0))
	/*
		Get data from cache
	*/
	var cachedData *models.MultiInstanceRawData
	if data, ok := cache.GetData(metaData); ok {
		if cachedData, ok = data.(*models.MultiInstanceRawData); ok {
			finalData[len(finalData)] = cachedData
		}
		logger.Info("Cached Number of entries", getNrForSingleEntry(cachedData.Data.Instances))
	}

	/*
		Get latest data. expected more data than in cache
	*/
	lenTillCached := len(finalData)
	finalData = initApiCallsAndAccomulateResponse(appendTimeRangeForApiCall, finalData, queryModel, metaData, authSettings, pluginSettings, logger)
	logger.Info("Append Number of entries", getNrOfEntries(finalData, lenTillCached))
	logger.Info("Total Number of entries", getNrOfEntries(finalData, 0))
	if len(finalData) == 0 {
		if response.Error == nil {
			response.Error = errors.New(constants.NoDataFromLM)
		}
	} else {
		response = processFinalData(queryModel, metaData, query.TimeRange.From.Unix(), query.TimeRange.To.Unix(), finalData, response, logger)
		logger.Info("size of data in bytes", cache.GetRealSize(metaData))
	}
	//TODO remove start
	logger.Info("")
	//TODO remove end

	return response
}

/*
Initiate goroutines to call API for each time range caclulated
*/
func initApiCallsAndAccomulateResponse(timeRangeForApiCall []models.PendingTimeRange, rawDataMap map[int]*models.MultiInstanceRawData,
	queryModel models.QueryModel, metaData models.MetaData, authSettings *models.AuthSettings, pluginSettings *models.PluginSettings,
	logger log.Logger) map[int]*models.MultiInstanceRawData {
	if len(timeRangeForApiCall) > 0 {
		dataLenIdx := len(rawDataMap)
		jobs := make(chan Job, len(timeRangeForApiCall))
		results := make(chan models.MultiInstanceRawData, len(timeRangeForApiCall))
		for i := 0; i < len(timeRangeForApiCall); i++ {
			go callDataAPI(jobs, results, &queryModel, pluginSettings, authSettings, metaData, logger)
		}
		for i := 0; i < len(timeRangeForApiCall); i++ {
			jobs <- Job{JobId: dataLenIdx, TimeFrom: timeRangeForApiCall[i].From, TimeTo: timeRangeForApiCall[i].To}
			dataLenIdx++
		}
		close(jobs)
		for i := len(timeRangeForApiCall); i > 0; i-- {
			result := <-results
			rawDataMap[result.JobId] = &result
		}
		close(results)
	}
	return rawDataMap
}

// Gets fresh data by calling rest API
func callDataAPI(jobs chan Job, results chan<- models.MultiInstanceRawData, queryModel *models.QueryModel, pluginSettings *models.PluginSettings,
	authSettings *models.AuthSettings, metaData models.MetaData, logger log.Logger) {
	for job := range jobs {
		var rawData models.MultiInstanceRawData //nolint:exhaustivestruct
		rawData.JobId = job.JobId
		rawData.FromTime = job.TimeFrom
		rawData.ToTime = job.TimeTo
		fullPath := BuildURLReplacingQueryParams(constants.RawDataMultiInstanceReq, queryModel, job.TimeFrom, job.TimeTo, metaData)

		logger.Debug("Calling API  => ", pluginSettings.Path, fullPath)
		//todo remove the loggers

		respByte, err := httpclient.Get(pluginSettings, authSettings, fullPath, constants.RawDataMultiInstanceReq, logger)
		if err != nil {
			rawData.Error = err.Error()
			logger.Error("Error from server => ", err)
		} else {
			err = json.Unmarshal(respByte, &rawData)
			if err != nil {
				rawData.Error = err.Error()
				logger.Error(constants.ErrorUnmarshallingErrorData+"raw-data => ", err)
			}
		}
		results <- rawData
	}
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
		cache.SetFirstTimeStamp(metaData, rawDataMap[k].FromTime)
		for instanceName, valueAndTime := range rawDataMap[k].Data.Instances {
			// Check if instance selected/regex matching
			shortenInstance, matched := IsInstanceMatched(metaData, &queryModel, rawDataMap[k].Data.DataSourceName, instanceName)
			if matched {
				if len(valueAndTime.Time) > 0 {
					cache.SetLastTimeStamp(metaData, time.UnixMilli(valueAndTime.Time[0]).Unix())
				}
				metaData.MatchedInstances = true
				var frame *data.Frame
				dataPontMap := make(map[string]int)
				frame = getFrame(dataFrameMap, shortenInstance, queryModel.DataPointSelected)
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
	// Check for errors, add franmes to response and store data in cache
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
		logger.Info("Cache size (same as number of panels)", cache.GetCount())
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
