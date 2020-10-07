package scache

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"
)

var (
	ErrNotFound             = errors.New("not found")
	ErrKeyIsNil             = errors.New("key is nil")
	ErrInvlidKeyTypeForHash = errors.New("invalid key type for hash function")
)

type LoadFunc func(key interface{}) (value interface{}, err error)

type Cache struct {
	maxShardIndex uint64
	shards        []iShard
	counter       *counter
	itemsToPrune  uint32
	timer         *timer
	wg            sync.WaitGroup
	ctx           context.Context
	ctxCancel     func()
	chClean       chan struct{}
}

func (c *Cache) Close() {
	c.ctxCancel()
	c.wg.Wait()
}

func (c *Cache) Set(key interface{}, value interface{}) {

	bID, err := c.shardID(key)
	if err == nil {
		c.shards[bID].Set(key, value)
	}
}

func (c *Cache) SetExp(key interface{}, value interface{}, ttl time.Duration) {

	bID, err := c.shardID(key)
	if err == nil {
		c.shards[bID].SetExp(key, value, ttl)
	}
}

func (c *Cache) Get(key interface{}) (value interface{}, err error) {

	bID, err := c.shardID(key)
	if err == nil {
		value, err = c.shards[bID].Get(key)
	}

	return
}

func (c *Cache) Del(key interface{}) (ok bool) {

	bID, err := c.shardID(key)
	if err == nil {
		ok = c.shards[bID].Del(key)
	}

	return
}

func (c *Cache) Count() (count int64) {
	return c.counter.Count()
}

func (c *Cache) runCleaner() {

	c.wg.Add(1)

	go func() {
		defer c.wg.Done()

		var (
			ok          bool
			removed     int
			expiredKeys = make([]interface{}, 0, 10000)
			oldest      = newListWithOldEntriesLRU(int(c.itemsToPrune))
		)

		for {
			select {
			case <-c.ctx.Done():
				return
			case <-c.chClean:
			}

			if removed > 0 {
				removed -= 1
				continue
			}

			oldest.Clear()

			for _, s := range c.shards {
				expiredKeys = expiredKeys[:0]
				s.GetForRemove(&expiredKeys, oldest)
				for _, k := range expiredKeys {
					if k != nil {
						if ok = c.Del(k); ok {
							removed++
						}
					}
				}
			}

			// if
			if removed == 0 {
				for _, k := range oldest.Keys() {
					if k != nil {
						c.Del(k)
						removed++
					}
				}
			}
		}
	}()
}

func (c *Cache) shardID(key interface{}) (id int, err error) {

	if key == nil {
		err = ErrKeyIsNil
		log.Println("failed to get key hash", err)
		return
	}

	if c.maxShardIndex == 0 {
		return // nothing do. it's the fastests case.
	}

	const prime64 = 1099511628211

	var val uint64

	switch src := key.(type) {
	case string:
		// copy from https://golang.org/pkg/hash/fnv/#New64 (see method 'Write')
		for _, symbol := range src {
			val ^= uint64(symbol)
			val *= prime64
		}

	case uint8:
		val = uint64(src)
	case uint16:
		val = uint64(src)
	case uint32:
		val = uint64(src)
	case uint64:
		val = src
	case int8:
		val = uint64(src)
	case int16:
		val = uint64(src)
	case int32:
		val = uint64(src)
	case int64:
		val = uint64(src)
	case float32:
		val = uint64(src)
	case float64:
		val = uint64(src)
	case int:
		val = uint64(src)
	case uint:
		val = uint64(src)
	default:
		err = ErrInvlidKeyTypeForHash
		log.Println("failed to get key hash", err)
		return

	}

	id = int(val & c.maxShardIndex)

	return
}
