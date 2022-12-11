package logicmonitor

import (
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/grafana/grafana-logicmonitor-datasource-backend/pkg/cache"
	"github.com/grafana/grafana-logicmonitor-datasource-backend/pkg/constants"
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
func GetData(query backend.DataQuery, queryModel models.QueryModel, metaData models.MetaData, authSettings *models.AuthSettings,
	pluginSettings *models.PluginSettings, pluginContext backend.PluginContext, logger log.Logger) backend.DataResponse {

	response := backend.DataResponse{}
	finalData := make(map[int]*models.MultiInstanceRawData)

	//TODO remove start
	logger.Info("")
	logger.Info("ID", metaData.Id)
	logger.Info("IsCallFromQueryEditor", metaData.EditMode)
	logger.Info("First Entry TimeStamp", time.UnixMilli(cache.GetFirstRawDataEntryTimestamp(metaData, queryModel.EnableDataAppendFeature)*1000))
	logger.Info("Last Entry TimeStamp", time.UnixMilli(cache.GetLastestRawDataEntryTimestamp(metaData, queryModel.EnableDataAppendFeature)*1000))
	//TODO remove end

	/*
		Step 1
		1. wait time is over/requets then calculate time range. Expect a new data if timeRangeForApiCall has entry
		2. Caclulate time range for rate limits records, multiple call will be made to each time range
	*/
	response, prependTimeRangeForApiCall, appendTimeRangeForApiCall := getTimeRange(query, queryModel, metaData, pluginContext, response, logger)

	/*
		Get earlier data than what is already in the cache
	*/
	finalData = getDataFromApi(prependTimeRangeForApiCall, finalData, queryModel, metaData, authSettings, pluginSettings, logger)
	logger.Info("Prepend Nr Of Entries", getNrOfEntries(finalData))
	/*
		Get data from cache
	*/
	var cachedData *models.MultiInstanceRawData
	if data, ok := cache.GetData(metaData); ok {
		dataMapLen := len(finalData)
		if cachedData, ok = data.(*models.MultiInstanceRawData); ok {
			finalData[dataMapLen] = cachedData
			dataMapLen++
		}
		logger.Info("Cached Number of entries", getNrForSingleEntry(cachedData.Data.Instances))
	}

	/*
		Get latest data. expected more data than in cache
	*/
	tempLen := len(finalData)
	finalData = getDataFromApi(appendTimeRangeForApiCall, finalData, queryModel, metaData, authSettings, pluginSettings, logger)
	logger.Info("Append Number of entries", getNrOfEntries(finalData))
	logger.Info("Total Number of entries", getNrOfEntries(finalData)-tempLen)

	if len(finalData) == 0 {
		response.Error = errors.New(constants.NoDataFromLM)
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
1. Initiate goroutines to call API for each time range caclulated
*/
func getDataFromApi(timeRangeForApiCall []models.PendingTimeRange, rawDataMap map[int]*models.MultiInstanceRawData, queryModel models.QueryModel,
	metaData models.MetaData, authSettings *models.AuthSettings, pluginSettings *models.PluginSettings,
	logger log.Logger) map[int]*models.MultiInstanceRawData {
	if len(timeRangeForApiCall) > 0 {
		wg := &sync.WaitGroup{}
		dataLenIdx := len(rawDataMap)
		// nrOfConcurrentJobs := 5
		jobs := make(chan Job, len(timeRangeForApiCall))
		for i := 0; i < len(timeRangeForApiCall); i++ {
			wg.Add(1)
			go CallDataAPI(wg, jobs, rawDataMap, &queryModel, pluginSettings, authSettings, metaData, logger)
		}
		for i := 0; i < len(timeRangeForApiCall); i++ {
			jobs <- Job{JobId: dataLenIdx, TimeFrom: timeRangeForApiCall[i].From, TimeTo: timeRangeForApiCall[i].To}
			dataLenIdx++
		}
		close(jobs)
		wg.Wait()
	}
	return rawDataMap
}

func getTimeRange(query backend.DataQuery, queryModel models.QueryModel, metaData models.MetaData, pluginContext backend.PluginContext,
	response backend.DataResponse, logger log.Logger) (backend.DataResponse, []models.PendingTimeRange, []models.PendingTimeRange) {
	var prependTimeRangeForApiCall []models.PendingTimeRange
	var appendTimeRangeForApiCall []models.PendingTimeRange
	var firstRawDataEntryTimestamp int64
	if _, ok := cache.GetData(metaData); !ok {
		// No data in cache. data is deleted(ttl) so get fresh data
		firstRawDataEntryTimestamp = math.MaxInt64
	} else {
		firstRawDataEntryTimestamp = cache.GetFirstRawDataEntryTimestamp(metaData, queryModel.EnableDataAppendFeature)
	}
	needToPrependData := firstRawDataEntryTimestamp < math.MaxInt64 && firstRawDataEntryTimestamp-query.TimeRange.From.Unix() > queryModel.CollectInterval &&
		queryModel.EnableHistoricalData
	if needToPrependData {
		prependTimeRangeForApiCall = GetTimeRanges(UnixTruncateToNearestMinute(query.TimeRange.From.Unix(), 60),
			firstRawDataEntryTimestamp-1, queryModel.CollectInterval, metaData, logger)
	}
	waitSec := GetWaitTimeInSec(metaData, queryModel.CollectInterval, queryModel.EnableDataAppendFeature)
	needToAppendData := waitSec == 0
	if (needToAppendData || needToPrependData || response.Error != nil) && (queryModel.EnableDataAppendFeature) {
		var lastRawDataEntryTimestamp int64
		if _, ok := cache.GetData(metaData); !ok {
			// No data in cache. data is deleted(ttl) so get fresh data
			lastRawDataEntryTimestamp = 0
		} else {
			lastRawDataEntryTimestamp = cache.GetLastestRawDataEntryTimestamp(metaData, queryModel.EnableDataAppendFeature)
		}
		if lastRawDataEntryTimestamp > 0 {
			lastRawDataEntryTimestamp++ // LastTimeStamp, increase by 1 second
		} else {
			lastRawDataEntryTimestamp = UnixTruncateToNearestMinute(query.TimeRange.From.Unix(), 60)
		}
		if queryModel.EnableHistoricalData {
			appendTimeRangeForApiCall = GetTimeRanges(lastRawDataEntryTimestamp, query.TimeRange.To.Unix(), queryModel.CollectInterval, metaData, logger)
		} else {
			appendTimeRangeForApiCall = append(appendTimeRangeForApiCall, models.PendingTimeRange{
				From: lastRawDataEntryTimestamp,
				To:   query.TimeRange.To.Unix()})
		}
		if len(prependTimeRangeForApiCall) == 0 && len(appendTimeRangeForApiCall) == 0 {
			if metaData.IsForLastXTime {
				logger.Warn("Got no TimeRange for API call, try again, it should work!")
			}
		} else {
			apisCallsSofar := cache.GetApiCalls(pluginContext.DataSourceInstanceSettings.UID).NrOfCalls
			totalApis := apisCallsSofar + len(prependTimeRangeForApiCall) + len(appendTimeRangeForApiCall)
			allowedNrOfCalls := constants.NumberOfRecordsWithRateLimit - apisCallsSofar
			if totalApis > constants.NumberOfRecordsWithRateLimit {
				logger.Error(fmt.Sprintf(constants.RateLimitAuditMsg, apisCallsSofar,
					len(prependTimeRangeForApiCall)+len(appendTimeRangeForApiCall), totalApis, allowedNrOfCalls))
				response.Error = fmt.Errorf(constants.RateLimitAuditMsg, apisCallsSofar,
					len(prependTimeRangeForApiCall)+len(appendTimeRangeForApiCall), totalApis, allowedNrOfCalls)
			} else {
				cache.AddApiCalls(pluginContext.DataSourceInstanceSettings.UID, totalApis)
			}
			logger.Info(fmt.Sprintf(constants.RateLimitValidation, apisCallsSofar, len(prependTimeRangeForApiCall)+len(appendTimeRangeForApiCall), totalApis))
		}
	} else if queryModel.EnableDataAppendFeature {
		logger.Info("Waiting seconds for next data", waitSec)
	}
	return response, prependTimeRangeForApiCall, appendTimeRangeForApiCall
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
			return response
		}
		for instanceName, valueAndTime := range rawDataMap[k].Data.Instances {
			// Check if instance selected/regex matching
			shortenInstance, matched := IsInstanceMatched(metaData, &queryModel, rawDataMap[k].Data.DataSourceName, instanceName)
			if matched {
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
			// Set First and Last time of all records
			SetFirstTimeStamp(queryModel, metaData, valueAndTime, logger)
			SetLastTimeStamp(queryModel, metaData, valueAndTime, logger)
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
		logger.Info("Cache size (same as number of panels unless deleted as per ttl)", cache.GetCount())
	}
	return response
}

// Set First record TimeStamp
func SetFirstTimeStamp(queryModel models.QueryModel, metaData models.MetaData, valueAndTime models.ValuesAndTime, logger log.Logger) {
	if queryModel.EnableDataAppendFeature {
		if len(valueAndTime.Time) > 0 {
			firstTimeOfAllInstances := time.UnixMilli(valueAndTime.Time[len(valueAndTime.Time)-1]).Unix()
			if cache.GetFirstRawDataEntryTimestamp(metaData, queryModel.EnableDataAppendFeature) > firstTimeOfAllInstances {
				cache.StoreFirstRawDataEntryTimestamp(metaData, firstTimeOfAllInstances)
			}
		}
	}
}

// Set Last record TimeStamp
func SetLastTimeStamp(queryModel models.QueryModel, metaData models.MetaData, valueAndTime models.ValuesAndTime, logger log.Logger) {
	if queryModel.EnableDataAppendFeature {
		if len(valueAndTime.Time) > 0 {
			latestTimeOfAllInstances := time.UnixMilli(valueAndTime.Time[0]).Unix()
			if cache.GetLastestRawDataEntryTimestamp(metaData, queryModel.EnableDataAppendFeature) < latestTimeOfAllInstances {
				cache.StoreLastestRawDataEntryTimestamp(metaData, latestTimeOfAllInstances)
			}
		}
	}
}

func getNrOfEntries(data map[int]*models.MultiInstanceRawData) int {
	tot := 0
	if len(data) > 0 {
		for i := len(data) - 1; i >= 0; i-- {
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
