package cache

import (
	"fmt"
	"time"

	"github.com/ReneKroon/ttlcache"
	"github.com/grafana/grafana-logicmonitor-datasource-backend/pkg/constants"
)

// Stores mapping of host data source id against ket host and datasource. caching this mapping avoids multiple API call for when host variable is changed
var hostDsAndHdsMapping = ttlcache.NewCache() //nolint:gochecknoglobals

func GetHdsByHostAndDs(host string, ds int64) (int64, bool) {
	if v, ok := hostDsAndHdsMapping.Get(fmt.Sprintf("%s-%d", host, ds)); ok {
		return v.(int64), true
	}
	return 0, false
}

func StoreHds(host string, ds int64, hds int64) {
	hostDsAndHdsMapping.SetWithTTL(fmt.Sprintf("%s-%d", host, ds), hds, time.Duration(constants.HostDsAndHdsMappingCacheTTLInMinutes*60)*time.Second)
}
