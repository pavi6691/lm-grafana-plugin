package cache

import (
	"bytes"
	"encoding/gob"
	"time"

	"github.com/ReneKroon/ttlcache"
	"github.com/grafana/grafana-logicmonitor-datasource-backend/pkg/models"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
)

// queryEditorTempCache whole raw data response and is used while making selection query editor.
// this avoids multiple http calls while making selection.
var rawDataCache = ttlcache.NewCache() //nolint:gochecknoglobals

func GetData(metaData models.MetaData) (interface{}, bool) {
	if _, ok := rawDataCache.Get(metaData.Id); !ok {
		if v, ok := rawDataCache.Get(metaData.QueryId); ok {
			// copy data with query id to ID, Data with ID holds only necessory data not all
			rawDataCache.SetWithTTL(metaData.Id, v, time.Duration(metaData.CacheTTLInSeconds)*time.Second)
			rawDataCache.Remove(metaData.QueryId)
		}
	}
	return rawDataCache.Get(metaData.Id)
}

func GetCount() int {
	return rawDataCache.Count()
}

func GetRealSize(metaData models.MetaData) int {
	b := new(bytes.Buffer)
	v, ok := GetData(metaData)
	if ok {
		if err := gob.NewEncoder(b).Encode(v); err != nil {
			return 0
		}
	}
	return b.Len()
}

func StoreData(metaData models.MetaData, rawDataMap *models.MultiInstanceRawData) {
	rawDataCache.SetWithTTL(metaData.Id, rawDataMap, time.Duration(metaData.CacheTTLInSeconds)*time.Second)
}

func StoreDataAt(metaData models.MetaData, presentAt int, newData *models.MultiInstanceRawData, logger log.Logger) {
	rawDataMap := make(map[int]*models.MultiInstanceRawData)
	if data, ok := GetData(metaData); ok {
		rawDataMap = data.(map[int]*models.MultiInstanceRawData)
		if _, ok := rawDataMap[presentAt]; ok {
			rawDataMap[presentAt] = newData
		} else {
			rawDataMap[len(rawDataMap)] = newData
		}
		logger.Info("Size of map is ", len(rawDataMap))
	} else {
		rawDataMap[0] = newData
	}
	rawDataCache.SetWithTTL(metaData.Id, rawDataMap, time.Duration(metaData.CacheTTLInSeconds)*time.Second)
}

func StoreAdditionalDataAt(index int, dataToAdd *models.MultiInstanceRawData, rawDataMap map[int]*models.MultiInstanceRawData) map[int]*models.MultiInstanceRawData {
	if index < len(rawDataMap)-1 {
		for i := len(rawDataMap) - 1; i >= index; i-- {
			rawDataMap[i+1] = rawDataMap[i]
		}
	}
	rawDataMap[index] = dataToAdd
	return rawDataMap
}

func IsDataForTimeRangePresentIncCache(metaData models.MetaData, from int64, to int64, logger log.Logger) (bool, int) {
	if data, ok := GetData(metaData); ok {
		logger.Info("001")
		rawDataMap := data.(map[int]*models.MultiInstanceRawData)
		if len(rawDataMap) > 0 {
			logger.Info("002")
			for k := 0; k < len(rawDataMap); k++ {
				logger.Info("003")
				for _, valueAndTime := range rawDataMap[k].Data.Instances {
					logger.Info("004")
					firstEntryTime := time.UnixMilli(valueAndTime.Time[len(valueAndTime.Time)-1]).Unix()
					lastEntryTime := time.UnixMilli(valueAndTime.Time[0]).Unix()
					logger.Info("GetFirstRawDataEntryTimestamp => ", time.UnixMilli(GetFirstRawDataEntryTimestamp(metaData, true)*1000))
					logger.Info("firstEntryTime => ", time.UnixMilli(firstEntryTime*1000))
					logger.Info("GetLastestRawDataEntryTimestamp => ", time.UnixMilli(GetLastestRawDataEntryTimestamp(metaData, true)*1000))
					logger.Info("LastEntryTime => ", time.UnixMilli(lastEntryTime*1000))
					if firstEntryTime-from > 300 {
						logger.Error("From  => ", time.UnixMilli(from*1000))
						logger.Error("firstEntryTime-from => ", firstEntryTime-from)
						return false, -1
					} else {
						return true, k
					}
				}
			}
		}
	}
	return false, -1
}
