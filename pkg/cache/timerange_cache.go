package cache

import (
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/ReneKroon/ttlcache"
	"github.com/grafana/grafana-logicmonitor-datasource-backend/pkg/constants"
	"github.com/grafana/grafana-logicmonitor-datasource-backend/pkg/models"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
)

/*
	Stores Timerange of API calls made so far and Nr of Api calls at current minutes. Based on this data,
	Calculates timeranges for multiple API calls to get raw data from santaba
*/

var mutex sync.Mutex

// TimeRange of all Api calls made so far
var startTime = ttlcache.NewCache()
var endTime = ttlcache.NewCache()

// Track API calls made so far current minute
var apiCallsTracker sync.Map

type ApiCallsTracker struct {
	TimeStamp      int64
	NrOfCalls      int
	TotalNrOfCalls int
}

func GetTimeRanges(query backend.DataQuery, queryModel models.QueryModel, metaData models.MetaData, pluginContext backend.PluginContext,
	response backend.DataResponse, logger log.Logger) (backend.DataResponse, []models.PendingTimeRange, []models.PendingTimeRange, models.MetaData) {
	var prependTimeRangeForApiCall []models.PendingTimeRange
	var appendTimeRangeForApiCall []models.PendingTimeRange
	firstRawDataEntryTimestamp := getStartTime(metaData)
	lastRawDataEntryTimestamp := getEndTime(metaData)
	waitSec := checkToWait(metaData, query, queryModel)
	currentApiCalls := numberOfApiCalls(firstRawDataEntryTimestamp, query.TimeRange.To.Unix(), queryModel)
	if (waitSec == 0 || response.Error != nil) && (queryModel.MaxNumberOfApiCallPerQuery < 0 || queryModel.MaxNumberOfApiCallPerQuery > currentApiCalls) {
		if lastRawDataEntryTimestamp > 0 && queryModel.EnableStrategicApiCallFeature {
			lastRawDataEntryTimestamp++
		} else {
			lastRawDataEntryTimestamp = unixTruncateToNearestMinute(query.TimeRange.From.Unix(), 60)
		}
		getEearlierData := firstRawDataEntryTimestamp < math.MaxInt64 && firstRawDataEntryTimestamp-query.TimeRange.From.Unix() > queryModel.CollectInterval
		if queryModel.EnableHistoricalData {
			if getEearlierData {
				prependTimeRangeForApiCall, metaData = getTimeRanges(unixTruncateToNearestMinute(query.TimeRange.From.Unix(), 60),
					firstRawDataEntryTimestamp-1, queryModel, pluginContext, metaData, logger)
			} else {
				appendTimeRangeForApiCall, metaData = getTimeRanges(lastRawDataEntryTimestamp, query.TimeRange.To.Unix(),
					queryModel, pluginContext, metaData, logger)
			}
		} else {
			if getEearlierData {
				// restrict for only one API call as historical data is disabled
				if (((query.TimeRange.To.Unix() - query.TimeRange.From.Unix()) / 60) / (queryModel.CollectInterval / 60)) < constants.MaxNumberOfRecordsPerApiCall {
					prependTimeRangeForApiCall = append(prependTimeRangeForApiCall, models.PendingTimeRange{
						From: unixTruncateToNearestMinute(query.TimeRange.From.Unix(), 60),
						To:   firstRawDataEntryTimestamp - 1})
				}
			}
			if (query.TimeRange.To.Unix() - lastRawDataEntryTimestamp) > queryModel.CollectInterval {
				appendTimeRangeForApiCall = append(appendTimeRangeForApiCall, models.PendingTimeRange{
					From: lastRawDataEntryTimestamp,
					To:   query.TimeRange.To.Unix()})
			} else {
				waitSec = queryModel.CollectInterval - (query.TimeRange.To.Unix() - lastRawDataEntryTimestamp)
			}
			AddNrOfApiCalls(pluginContext.DataSourceInstanceSettings.UID, len(prependTimeRangeForApiCall)+len(appendTimeRangeForApiCall))
		}
	}
	if metaData.PendingApiCalls > 0 {
		logger.Warn(constants.RateLimitExceeding, metaData.PendingApiCalls)
		response.Error = fmt.Errorf(fmt.Sprintf(constants.RateLimitExceeding, metaData.PendingApiCalls))
	} else {
		if waitSec > 0 {
			logger.Info("Id", metaData.Id)
			logger.Info(constants.WaitingSecondsForNextData, waitSec)
		} else if len(prependTimeRangeForApiCall) == 0 && len(appendTimeRangeForApiCall) == 0 &&
			metaData.IsForLastXTime && queryModel.EnableStrategicApiCallFeature {
			logger.Warn(constants.NoTimeRangeError)
		}
	}
	// response = recordApiCallsSofarLastMinute(pluginContext, appendTimeRangeForApiCall, prependTimeRangeForApiCall, response, logger)
	return response, prependTimeRangeForApiCall, appendTimeRangeForApiCall, metaData
}

func getTimeRanges(timeRangeStart int64, timeRangeEnd int64, queryModel models.QueryModel, pluginContext backend.PluginContext,
	metaData models.MetaData, logger log.Logger) ([]models.PendingTimeRange, models.MetaData) {
	recordsToAppend := recordsToAppend(timeRangeStart, timeRangeEnd, queryModel)
	currentApiCalls := numberOfApiCalls(timeRangeStart, timeRangeEnd, queryModel)
	if recordsToAppend%constants.MaxNumberOfRecordsPerApiCall > 0 {
		currentApiCalls++
	}
	if queryModel.ConcurrentApiCallsPerQuery > 0 && currentApiCalls > queryModel.ConcurrentApiCallsPerQuery {
		currentApiCalls = queryModel.ConcurrentApiCallsPerQuery
	}
	var pendingTimeRange []models.PendingTimeRange
	mutex.Lock()
	logger.Info("")
	logger.Info("ID", metaData.Id)
	logger.Debug("Is in EditMode", metaData.EditMode)
	logger.Debug("First Entry TimeStamp", time.UnixMilli(getStartTime(metaData)*1000))
	logger.Debug("Last Entry TimeStamp", time.UnixMilli(getEndTime(metaData)*1000))
	logger.Debug("RecordsToAppend", recordsToAppend)
	logger.Info("Required Api Calls", currentApiCalls)
	pendingApiCalls := numberOfApiCalls(timeRangeStart, getStartTime(metaData), queryModel)
	if pendingApiCalls > 0 {
		logger.Error(constants.PendingApiCallsMsg, pendingApiCalls)
	}
	apisCallsSofar := GetNrOfApiCalls(pluginContext.DataSourceInstanceSettings.UID).NrOfCalls
	logger.Debug("Api calls so far this minute", apisCallsSofar)
	if queryModel.EnableApiCallThrottler && (currentApiCalls+int64(apisCallsSofar)) > constants.MaxApiCallsRateLimit {
		metaData.PendingApiCalls = (int(currentApiCalls) + apisCallsSofar) - constants.MaxApiCallsRateLimit
		currentApiCalls = constants.MaxApiCallsRateLimit - int64(apisCallsSofar)
	}
	logger.Info("Available nr of Api Calls", constants.MaxApiCallsRateLimit-int64(apisCallsSofar))
	logger.Info("Cache size (same as number of panels)", GetCount())
	pendingTimeRange = make([]models.PendingTimeRange, currentApiCalls)
	var call int64
	var from int64
	for call = currentApiCalls - 1; call >= 0; call-- {
		if recordsToAppend > constants.MaxNumberOfRecordsPerApiCall {
			from = timeRangeEnd - (constants.MaxNumberOfRecordsPerApiCall * queryModel.CollectInterval)
		} else {
			from = timeRangeEnd - (recordsToAppend * queryModel.CollectInterval)
		}
		if time.Now().Unix() < timeRangeEnd {
			pendingTimeRange[call] = models.PendingTimeRange{From: from, To: time.Now().Unix()}
			recordsToAppend = 0
			break
		} else {
			pendingTimeRange[call] = models.PendingTimeRange{From: from, To: timeRangeEnd}
		}
		recordsToAppend = recordsToAppend - constants.MaxNumberOfRecordsPerApiCall
		timeRangeEnd = from - 1
	}
	AddNrOfApiCalls(pluginContext.DataSourceInstanceSettings.UID, int(currentApiCalls))
	mutex.Unlock()
	return pendingTimeRange, metaData
}

func recordsToAppend(timeRangeStart int64, timeRangeEnd int64, queryModel models.QueryModel) int64 {
	return ((timeRangeEnd - timeRangeStart) / 60) / (queryModel.CollectInterval / 60)
}

func numberOfApiCalls(timeRangeStart int64, timeRangeEnd int64, queryModel models.QueryModel) int64 {
	recordsToAppend := recordsToAppend(timeRangeStart, timeRangeEnd, queryModel)
	return (recordsToAppend / constants.MaxNumberOfRecordsPerApiCall)
}

/*
Returns seconds to wait before making API call for new data. is based on ds collect interval time and last time when data is recieved.
*/
func checkToWait(metaData models.MetaData, query backend.DataQuery, queryModel models.QueryModel) int64 {
	secondsAfterLastData := (query.TimeRange.To.Unix() - getEndTime(metaData))
	secondsBeforeFirstData := getStartTime(metaData) - query.TimeRange.From.Unix()
	if secondsAfterLastData < queryModel.CollectInterval && secondsBeforeFirstData < queryModel.CollectInterval {
		return queryModel.CollectInterval - secondsAfterLastData
	}
	return 0
}

// Set First record TimeStamp
func SetFirstTimeStamp(metaData models.MetaData, time int64) {
	if time > 0 && getStartTime(metaData) > time {
		storeStartTime(metaData, time)
	}
}

// Set Last record TimeStamp
func SetLastTimeStamp(metaData models.MetaData, time int64) {
	if time > 0 && getEndTime(metaData) < time {
		storeEndTime(metaData, time)
	}
}

func GetNrOfApiCalls(key string) ApiCallsTracker {
	v, ok := apiCallsTracker.Load(key)
	if ok {
		apiCTrack := v.(ApiCallsTracker)
		if (apiCTrack.TimeStamp + 60) > time.Now().Unix() {
			return apiCTrack
		}
	}
	return ApiCallsTracker{}
}

func AddNrOfApiCalls(id string, currentApiCalls int) {
	apiCTrack := GetNrOfApiCalls(id)
	if (apiCTrack.TimeStamp + 60) > time.Now().Unix() {
		apiCallsTracker.Store(id, ApiCallsTracker{
			TimeStamp:      apiCTrack.TimeStamp,
			NrOfCalls:      apiCTrack.NrOfCalls + currentApiCalls,
			TotalNrOfCalls: apiCTrack.TotalNrOfCalls - currentApiCalls,
		})
	} else {
		apiCallsTracker.Store(id, ApiCallsTracker{
			TimeStamp:      unixTruncateToNearestMinute(time.Now().Unix(), 60),
			NrOfCalls:      apiCTrack.NrOfCalls + currentApiCalls,
			TotalNrOfCalls: apiCTrack.TotalNrOfCalls - currentApiCalls,
		})
	}
}

func unixTruncateToNearestMinute(inputTime int64, intervalMin int64) int64 {
	inputTimeTruncated := time.UnixMilli(inputTime * 1000).Truncate(time.Duration(intervalMin) * time.Second)

	return inputTimeTruncated.Unix()
}

func storeEndTime(metaData models.MetaData, timeStamp int64) {
	endTime.SetWithTTL(metaData.Id, timeStamp, time.Duration(metaData.CacheTTLInSeconds+60)*time.Second)
}

func storeStartTime(metaData models.MetaData, timeStamp int64) {
	startTime.SetWithTTL(metaData.Id, timeStamp, time.Duration(metaData.CacheTTLInSeconds+60)*time.Second)
}

func getEndTime(metaData models.MetaData) int64 {
	if v, ok := endTime.Get(metaData.Id); ok {
		return v.(int64)
	} else if v, ok := endTime.Get(metaData.QueryId); ok {
		if !metaData.EditMode {
			endTime.Set(metaData.Id, v.(int64))
			endTime.Remove(metaData.QueryId)
		}
		return v.(int64)
	}
	return 0
}

func getStartTime(metaData models.MetaData) int64 {
	if v, ok := startTime.Get(metaData.Id); ok {
		return v.(int64)
	} else if v, ok := startTime.Get(metaData.QueryId); ok {
		if !metaData.EditMode {
			startTime.Remove(metaData.QueryId)
			startTime.Set(metaData.Id, v.(int64))
		}
		return v.(int64)
	}
	return math.MaxInt64
}
