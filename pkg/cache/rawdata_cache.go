package cache

import (
	"time"

	"github.com/ReneKroon/ttlcache"
	"github.com/grafana/grafana-logicmonitor-datasource-backend/pkg/models"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data"
)

// frameDataCache store frame data. The TTL is till the Polling interval.
var frameDataCache = ttlcache.NewCache() //nolint:gochecknoglobals

// queryEditorTempCache whole raw data response and is used while making selection query editor.
// this avoids multiple http calls while making selection.
var queryEditorTempCache = ttlcache.NewCache() //nolint:gochecknoglobals

func GetData(key string) (interface{}, bool) {
	return frameDataCache.Get(key)
}

func GetFrameDataCount() int {
	return frameDataCache.Count()
}

func StoreFrame(key string, collectInterval int64, frame data.Frames) {
	frameDataCache.SetWithTTL(key, frame, time.Duration(collectInterval)*time.Second)
}

func GetQueryEditorCacheData(key string) (interface{}, bool) {
	return queryEditorTempCache.Get(key)
}

func GetQueryEditorCacheDataCount() int {
	return queryEditorTempCache.Count()
}

func StoreQueryEditorTempData(key string, collectInterval int64, data models.MultiInstanceData) {
	queryEditorTempCache.SetWithTTL(key, data, time.Duration(collectInterval)*time.Second)
}

// GetCurrentTimeRange return currentTimeRange in minutes.
// there can be duplicate panel with same from date and different toDate. So consider toDate as well
func GetCurrentTimeRange(query *backend.DataQuery) int64 {
	from := time.Unix(query.TimeRange.From.Unix(), 0)
	to := time.Unix(query.TimeRange.To.Unix(), 0)

	return from.Truncate(time.Minute).Unix() + to.Truncate(time.Minute).Unix()
}
