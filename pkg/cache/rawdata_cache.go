package cache

import (
	"time"

	"github.com/ReneKroon/ttlcache"
	"github.com/grafana/grafana-logicmonitor-datasource-backend/pkg/models"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
)

// frameDataCache store frame data. The TTL is till the Polling interval.
var frameDataCache = ttlcache.NewCache() //nolint:gochecknoglobals

// queryEditorTempCache whole raw data response and is used while making selection query editor.
// this avoids multiple http calls while making selection.
var queryEditorTempCache = ttlcache.NewCache() //nolint:gochecknoglobals

// Time stamp of rawdata recieved.
var lastRawDataEntryTimestamp = ttlcache.NewCache()

func GetData(id string) (interface{}, bool) {
	return frameDataCache.Get(id)
}

func GetFrameDataCount() int {
	return frameDataCache.Count()
}

// when this is called there no errors, so clear any previous error and store data
func StoreFrame(id string, ttl int64, response backend.DataResponse) {
	frameDataCache.SetWithTTL(id, response, time.Duration(ttl)*time.Second)
}

func GetFrameCount(key string) int {
	frameValue, ok := GetData(key)
	if ok {
		df, ok := frameValue.(backend.DataResponse)
		if ok {
			l, ok := df.Frames[0].RowLen()
			if ok == nil {
				return l
			}
		}
	}
	return 0
}

func StoreLastestRawDataEntryTimestamp(metaData models.MetaData, timeStamp int64, ttl int64) {
	if metaData.IsCallFromQueryEditor {
		lastRawDataEntryTimestamp.Set(metaData.TempQueryEditorID, timeStamp)
		lastRawDataEntryTimestamp.Set(metaData.FrameId, timeStamp)
	} else {
		lastRawDataEntryTimestamp.Set(metaData.FrameId, timeStamp)
	}
}

func GetLastestRawDataEntryTimestamp(metaData models.MetaData) int64 {
	if metaData.IsCallFromQueryEditor {
		if _, ok := GetQueryEditorCacheData(metaData.TempQueryEditorID); !ok {
			lastRawDataEntryTimestamp.Remove(metaData.TempQueryEditorID)
			return 0
		}
		if v, ok := lastRawDataEntryTimestamp.Get(metaData.TempQueryEditorID); ok {
			return v.(int64)
		}
	} else {
		if _, ok := GetQueryEditorCacheData(metaData.TempQueryEditorID); !ok {
			lastRawDataEntryTimestamp.Remove(metaData.TempQueryEditorID)
		}
		if _, ok := GetData(metaData.FrameId); !ok {
			lastRawDataEntryTimestamp.Remove(metaData.FrameId)
			return 0
		}
		if v, ok := lastRawDataEntryTimestamp.Get(metaData.FrameId); ok {
			return v.(int64)
		}
	}
	return 0
}

func GetQueryEditorCacheData(id string) (interface{}, bool) {
	return queryEditorTempCache.Get(id)
}

func GetQueryEditorCacheDataCount() int {
	return queryEditorTempCache.Count()
}

func StoreQueryEditorTempData(id string, ttl int64, rawDataMap map[int]*models.MultiInstanceRawData) {
	queryEditorTempCache.SetWithTTL(id, rawDataMap, time.Duration(ttl)*time.Second)
}

// GetCurrentTimeRange return currentTimeRange in minutes.
// there can be duplicate panel with same from date and different toDate. So consider toDate as well
func GetCurrentTimeRange(query *backend.DataQuery) int64 {
	from := time.Unix(query.TimeRange.From.Unix(), 0)
	to := time.Unix(query.TimeRange.To.Unix(), 0)

	return from.Truncate(time.Minute).Unix() + to.Truncate(time.Minute).Unix()
}
