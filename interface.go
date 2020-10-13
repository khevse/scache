package scache

import "time"

type ICache interface {
	// Set value with default lifetime(optional)
	Set(key interface{}, value interface{})
	// Set value with custom lifetime time
	SetExp(key interface{}, value interface{}, ttl time.Duration)
	Get(key interface{}) (value interface{}, err error)
	Del(key interface{}) bool
	Count() int64
	Close()
}

type iShard interface {
	// Set value with default lifetime(optional)
	Set(key interface{}, value interface{})
	// Set value with custom lifetime time
	SetExp(key interface{}, value interface{}, ttl time.Duration)
	Get(key interface{}) (value interface{}, err error)
	Del(key interface{}) bool
	Count() int64
	GetForRemove(expiredKeys *[]interface{}, oldest iListWithOldEntries)
}

type iListWithOldEntries interface {
	Add(key interface{}, itemCost *uint32, maxCost uint32)
}
