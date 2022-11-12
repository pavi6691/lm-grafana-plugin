package cache

import (
	"time"

	"github.com/ReneKroon/ttlcache"
	"github.com/grafana/grafana-logicmonitor-datasource-backend/pkg/models"
)

// queryEditorTempCache whole raw data response and is used while making selection query editor.
// this avoids multiple http calls while making selection.
var queryEditorTempCache = ttlcache.NewCache() //nolint:gochecknoglobals

func GetQueryEditorCacheData(id string) (interface{}, bool) {
	return queryEditorTempCache.Get(id)
}

func GetQueryEditorCacheDataCount() int {
	return queryEditorTempCache.Count()
}

func StoreQueryEditorTempData(id string, ttl int64, rawDataMap map[int]*models.MultiInstanceRawData) {
	queryEditorTempCache.SetWithTTL(id, rawDataMap, time.Duration(ttl)*time.Second)
}
