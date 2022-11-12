package cache

import (
	"time"

	"github.com/ReneKroon/ttlcache"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
)

// frameDataCache store frame data. The TTL is till the Polling interval.
var frameDataCache = ttlcache.NewCache() //nolint:gochecknoglobals

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
