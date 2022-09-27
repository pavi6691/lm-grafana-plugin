package cache

import (
	"time"

	"github.com/ReneKroon/ttlcache"
	"github.com/grafana/grafana-logicmonitor-datasource-backend/pkg/models"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data"
)

// store frame data that is resused util next interval
var frameDataCache = ttlcache.NewCache()

// temp cache store whole raw data response and is used while making selection query editor. this avoids multiple httpc calls while making selection.
// This gets deleted
var tempRawDataCache = ttlcache.NewCache()

var lastExecutedTimeCache = ttlcache.NewCache()

var lastTimeRangeCache = ttlcache.NewCache()

func GetLastExecutedTime(key string) (int64, bool) {
	lastExecutedTime, lastExecutedTimePresent := lastExecutedTimeCache.Get(key)
	var lastExecutedTimeInt = int64(0)
	if lastExecutedTimePresent {
		ok := false
		lastExecutedTimeInt, ok = lastExecutedTime.(int64)
		if !ok {
			return lastExecutedTimeInt, false
		}
	}
	return lastExecutedTimeInt, true
}

func GetLastTimeRange(key string) (interface{}, bool) {
	return lastTimeRangeCache.Get(key)
}

func GetData(key string) (interface{}, bool) {
	return frameDataCache.Get(key)
}

func RawDataCount() int {
	return frameDataCache.Count()
}

func StoreFrame(key string, collectInterval int64, query backend.DataQuery, frame data.Frames) {
	frameDataCache.SetWithTTL(key, frame, time.Duration(time.Duration(collectInterval)*time.Second))
	lastExecutedTimeCache.SetWithTTL(key, time.Now().UnixMilli(), time.Duration(time.Duration(collectInterval)*time.Second))
	lastTimeRangeCache.SetWithTTL(key, GetCurrentTimeRange(query), time.Duration(time.Duration(collectInterval)*time.Second))
}

func IsTempRawDataPresent(key string) bool {
	_, present := tempRawDataCache.Get(key)
	return present
}

func GetTempRawData(key string) (interface{}, bool) {
	return tempRawDataCache.Get(key)
}

func GetTempRawDataCount() int {
	return tempRawDataCache.Count()
}

func StoreTemp(key string, collectInterval int64, data models.MultiInstanceData) {
	tempRawDataCache.SetWithTTL(key, data, time.Duration(time.Duration(collectInterval+10)*time.Second))
}

// return currentTimeRange in minutes.
// there can be duplicate panel with same from date and differrent toDate. so concider toDate as well
func GetCurrentTimeRange(query backend.DataQuery) int64 {
	from := time.Unix(query.TimeRange.From.Unix(), 0)
	to := time.Unix(query.TimeRange.To.Unix(), 0)
	return from.Truncate(time.Minute).Unix() + to.Truncate(time.Minute).Unix()
}
