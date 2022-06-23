package machine

import "time"

// CacheTTL is duration of time to store the vm ip in cache
// Currently the sync period for Power VS machine api controller is 10 minutes that means every 10 minutes
// there will be a reconcilation on machine objects, So setting cache timeout to 20 minutes so the cache updates will happen
// once in 2 reconcilations.
const CacheTTL = time.Duration(20) * time.Minute

// vmIP holds the vm name and corresponding dhcp ip used to cache the dhcp ip
type vmIP struct {
	name string
	ip   string
}

// CacheKeyFunc defines the key function required in TTLStore.
func CacheKeyFunc(obj interface{}) (string, error) {
	return obj.(vmIP).name, nil
}
