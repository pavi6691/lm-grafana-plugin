package cache

import (
	"time"

	"github.com/ReneKroon/ttlcache"
)

var apiCallsTracker = ttlcache.NewCache()

type ApiCallsTracker struct {
	TimeStamp int64
	NrOfCalls int
}

func GetApiCalls(key string) ApiCallsTracker {
	apiCallsTracker.SkipTtlExtensionOnHit(true)
	v, ok := apiCallsTracker.Get(key)
	if ok {
		apiCTrack := v.(ApiCallsTracker)
		if (apiCTrack.TimeStamp + 60) > time.Now().Unix() {
			return apiCTrack
		}
	}
	return ApiCallsTracker{}
}

func AddApiCalls(id string, nrOfCalls int) {
	apiCTrack := GetApiCalls(id)
	if (apiCTrack.TimeStamp + 60) > time.Now().Unix() {
		apiCallsTracker.Set(id, ApiCallsTracker{TimeStamp: apiCTrack.TimeStamp, NrOfCalls: nrOfCalls})
	} else {
		apiCallsTracker.Set(id, ApiCallsTracker{TimeStamp: UnixTruncateToNearestMinute(time.Now().Unix(), 60), NrOfCalls: nrOfCalls})
	}
}
func UnixTruncateToNearestMinute(inputTime int64, intervalMin int64) int64 {
	inputTimeTruncated := time.UnixMilli(inputTime * 1000).Truncate(time.Duration(intervalMin) * time.Second)

	return inputTimeTruncated.Unix()
}
