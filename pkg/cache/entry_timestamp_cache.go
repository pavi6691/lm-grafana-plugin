package cache

import (
	"math"
	"time"

	"github.com/ReneKroon/ttlcache"
	"github.com/grafana/grafana-logicmonitor-datasource-backend/pkg/models"
)

// Time stamp of rawdata recieved.
var lastRawDataEntryTimestamp = ttlcache.NewCache()
var firstRawDataEntryTimestamp = ttlcache.NewCache()

func StoreLastestRawDataEntryTimestamp(metaData models.MetaData, timeStamp int64) {
	lastRawDataEntryTimestamp.SetWithTTL(metaData.Id, timeStamp, time.Duration(metaData.CacheTTLInSeconds+60)*time.Second)
}

func StoreFirstRawDataEntryTimestamp(metaData models.MetaData, timeStamp int64) {
	firstRawDataEntryTimestamp.SetWithTTL(metaData.Id, timeStamp, time.Duration(metaData.CacheTTLInSeconds+60)*time.Second)
}

func GetLastestRawDataEntryTimestamp(metaData models.MetaData, enableDataAppendFeature bool) int64 {
	if enableDataAppendFeature {
		if v, ok := lastRawDataEntryTimestamp.Get(metaData.Id); ok {
			return v.(int64)
		} else if v, ok := lastRawDataEntryTimestamp.Get(metaData.QueryId); ok {
			if !metaData.IsCallFromQueryEditor {
				lastRawDataEntryTimestamp.Set(metaData.Id, v.(int64))
				lastRawDataEntryTimestamp.Remove(metaData.QueryId)
			}
			return v.(int64)
		}
	}
	return 0
}

func GetFirstRawDataEntryTimestamp(metaData models.MetaData, enableDataAppendFeature bool) int64 {
	if enableDataAppendFeature {
		if v, ok := firstRawDataEntryTimestamp.Get(metaData.Id); ok {
			return v.(int64)
		} else if v, ok := firstRawDataEntryTimestamp.Get(metaData.QueryId); ok {
			if !metaData.IsCallFromQueryEditor {
				firstRawDataEntryTimestamp.Remove(metaData.QueryId)
				firstRawDataEntryTimestamp.Set(metaData.Id, v.(int64))
			}
			return v.(int64)
		}
	}
	return math.MaxInt64
}
