package cache

import (
	"time"

	"github.com/ReneKroon/ttlcache"
	"github.com/grafana/grafana-logicmonitor-datasource-backend/pkg/models"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data"
)

var cacheData = ttlcache.NewCache()

var lastExecutedTime = ttlcache.NewCache()

var timeRangeChanged = ttlcache.NewCache()

func GetLastExecutedTime(UniqueId string) (interface{}, bool) {
	return lastExecutedTime.Get(UniqueId)
}

func GetTimeRangeChanged(UniqueId string) (interface{}, bool) {
	return timeRangeChanged.Get(UniqueId)
}

func GetData(UniqueId string) (interface{}, bool) {
	return cacheData.Get(UniqueId)
}

func RawDataCount() int {
	return cacheData.Count()
}

func Store(qm models.QueryModel, query backend.DataQuery, frame *data.Frame) {
	cacheData.SetWithTTL(qm.UniqueID, frame, time.Duration(time.Duration(qm.CollectInterval+10)*time.Second))
	lastExecutedTime.SetWithTTL(qm.UniqueID, time.Now().UnixMilli(), time.Duration(time.Duration(qm.CollectInterval+10)*time.Second))
	timeRangeChanged.SetWithTTL(qm.UniqueID, query.TimeRange.Duration(), time.Duration(time.Duration(qm.CollectInterval+10)*time.Second))
}
