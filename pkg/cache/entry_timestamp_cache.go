package cache

import (
	"math"

	"github.com/ReneKroon/ttlcache"
	"github.com/grafana/grafana-logicmonitor-datasource-backend/pkg/models"
)

// Time stamp of rawdata recieved.
var lastRawDataEntryTimestamp = ttlcache.NewCache()
var firstRawDataEntryTimestamp = ttlcache.NewCache()

func StoreLastestRawDataEntryTimestamp(metaData models.MetaData, timeStamp int64, ttl int64) {
	if metaData.IsCallFromQueryEditor {
		lastRawDataEntryTimestamp.Set(metaData.TempQueryEditorID, timeStamp)
		lastRawDataEntryTimestamp.Set(metaData.FrameId, timeStamp)
	} else {
		lastRawDataEntryTimestamp.Set(metaData.FrameId, timeStamp)
	}
}

func StoreFirstRawDataEntryTimestamp(metaData models.MetaData, timeStamp int64, ttl int64) {
	if metaData.IsCallFromQueryEditor {
		firstRawDataEntryTimestamp.Set(metaData.TempQueryEditorID, timeStamp)
		firstRawDataEntryTimestamp.Set(metaData.FrameId, timeStamp)
	} else {
		firstRawDataEntryTimestamp.Set(metaData.FrameId, timeStamp)
	}
}

func GetLastestRawDataEntryTimestamp(metaData models.MetaData, enableDataAppendFeature bool) int64 {
	if enableDataAppendFeature {
		if metaData.IsCallFromQueryEditor {
			if _, ok := GetData(metaData.TempQueryEditorID); !ok {
				lastRawDataEntryTimestamp.Remove(metaData.TempQueryEditorID)
				return 0
			}
			if v, ok := lastRawDataEntryTimestamp.Get(metaData.TempQueryEditorID); ok {
				return v.(int64)
			}
		} else {
			if _, ok := GetData(metaData.TempQueryEditorID); !ok {
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
	}
	return 0
}

func GetFirstRawDataEntryTimestamp(metaData models.MetaData, enableDataAppendFeature bool) int64 {
	if enableDataAppendFeature {
		if metaData.IsCallFromQueryEditor {
			if _, ok := GetData(metaData.TempQueryEditorID); !ok {
				firstRawDataEntryTimestamp.Remove(metaData.TempQueryEditorID)
				return math.MaxInt64
			}
			if v, ok := firstRawDataEntryTimestamp.Get(metaData.TempQueryEditorID); ok {
				return v.(int64)
			}
		} else {
			if _, ok := GetData(metaData.TempQueryEditorID); !ok {
				firstRawDataEntryTimestamp.Remove(metaData.TempQueryEditorID)
			}
			if _, ok := GetData(metaData.FrameId); !ok {
				firstRawDataEntryTimestamp.Remove(metaData.FrameId)
				return math.MaxInt64
			}
			if v, ok := firstRawDataEntryTimestamp.Get(metaData.FrameId); ok {
				return v.(int64)
			}
		}
	}
	return math.MaxInt64
}
