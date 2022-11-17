package cache

import (
	"time"

	"github.com/ReneKroon/ttlcache"
	"github.com/grafana/grafana-logicmonitor-datasource-backend/pkg/models"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
)

// queryEditorTempCache whole raw data response and is used while making selection query editor.
// this avoids multiple http calls while making selection.
var rawDataCache = ttlcache.NewCache() //nolint:gochecknoglobals

func GetData(id string) (interface{}, bool) {
	return rawDataCache.Get(id)
}

func GetCount() int {
	return rawDataCache.Count()
}

func StoreData(id string, ttl int64, rawDataMap *models.MultiInstanceRawData) {
	rawDataCache.SetWithTTL(id, rawDataMap, time.Duration(ttl)*time.Second)
}

func StoreDataAt(id string, ttl int64, presentAt int, newData *models.MultiInstanceRawData, logger log.Logger) {
	rawDataMap := make(map[int]*models.MultiInstanceRawData)
	if data, ok := GetData(id); ok {
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
	rawDataCache.SetWithTTL(id, rawDataMap, time.Duration(ttl)*time.Second)
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
	if data, ok := GetData(metaData.FrameId); ok {
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

func GetDataFor(metaData models.MetaData, from int64, to int64, newRawDataMap map[int]*models.MultiInstanceRawData, logger log.Logger) *models.MultiInstanceRawData {
	var result *models.MultiInstanceRawData
	tempInstanceData := make(map[string]models.ValuesAndTime)
	// var vt models.ValuesAndTime
	for k := 0; k < len(newRawDataMap); k++ {
		// logger.Info("2")
		if newRawDataMap[k].Error != "OK" {
			// logger.Info("3")
			return &models.MultiInstanceRawData{Data: models.MultiInstanceData{DataSourceName: newRawDataMap[0].Data.DataSourceName, DataPoints: newRawDataMap[0].Data.DataPoints, Instances: tempInstanceData}, Error: newRawDataMap[k].Error}
		}
		// logger.Info("4")
		for instanceName, valueAndTime := range newRawDataMap[k].Data.Instances {
			// logger.Info("5")
			var Timet []int64
			var Valuest [][]interface{}
			for i := 0; i < len(valueAndTime.Time); i++ {
				// logger.Info("6")
				t := time.UnixMilli(valueAndTime.Time[i]).Unix()
				if from <= t && to >= t {
					Timet = append(Timet, valueAndTime.Time[i])
					Valuest = append(Valuest, valueAndTime.Values[i])
				} else if len(tempInstanceData) > 0 {
					break
				}
			}
			if _, ok := tempInstanceData[instanceName]; ok {
				// logger.Info("Sucess")
				tempInstanceData[instanceName] = models.ValuesAndTime{Time: append(Timet, tempInstanceData[instanceName].Time...), Values: append(Valuest, tempInstanceData[instanceName].Values...)}
			} else if len(Timet) > 0 && len(Valuest) > 0 {
				tempInstanceData[instanceName] = models.ValuesAndTime{Time: Timet, Values: Valuest}
			}
			// vt = models.ValuesAndTime{Time: Timet, Values: Valuest}
		}
	}
	// logger.Info("")
	// for _, v := range vt.Time {
	// 	logger.Info("time => ", time.UnixMilli(v))
	// }
	// logger.Info("7")
	if len(tempInstanceData) > 0 {
		// logger.Info("8")
		result = &models.MultiInstanceRawData{Data: models.MultiInstanceData{DataSourceName: newRawDataMap[0].Data.DataSourceName, DataPoints: newRawDataMap[0].Data.DataPoints, Instances: tempInstanceData}, Error: "OK"}
	}
	return result
}
