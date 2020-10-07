package scache

import (
	"sync"
	"sync/atomic"
	"time"
)

type itemLRU struct {
	Value  interface{}
	Expire int64
	Cost   *uint32
}

type shardRU struct {
	ttl          time.Duration
	itemsToPrune uint32
	counter      *counter
	timer        *timer
	payload      map[interface{}]*itemLRU
	loadFunc     LoadFunc
	mu           sync.RWMutex
	chClean      chan struct{}
}

func newShardRU(chClean chan struct{}, counter *counter, tm *timer, conf *Config, loadFunc LoadFunc) *shardRU {
	return &shardRU{
		ttl:          conf.TTL,
		itemsToPrune: conf.ItemsToPrune,
		payload:      make(map[interface{}]*itemLRU),
		loadFunc:     loadFunc,
		counter:      counter,
		timer:        tm,
		chClean:      chClean,
	}
}

func (s *shardRU) Count() (val int64) {
	s.mu.RLock()
	val = int64(len(s.payload))
	s.mu.RUnlock()
	return
}

func (s *shardRU) Set(key interface{}, value interface{}) {
	s.setExp(true, key, value, 0)
}

func (s *shardRU) SetExp(key interface{}, value interface{}, ttl time.Duration) {
	s.setExp(true, key, value, ttl)
}

func (s *shardRU) Get(key interface{}) (value interface{}, err error) {

	s.mu.RLock()
	elem, exist := s.payload[key]
	s.mu.RUnlock()

	if exist {
		cost := s.timer.Tick()
		atomic.StoreUint32(elem.Cost, cost)
		if elem.Expire != 0 && elem.Expire < timeNowLRU(0) {
			s.Del(key)
		} else {
			value = elem.Value
			return
		}
	}

	if s.loadFunc != nil {
		s.mu.Lock()
		elem, exist := s.payload[key]
		if exist {
			// if has already loaded
			value = elem.Value
			s.mu.Unlock()
			return
		}

		value, err = s.loadFunc(key)
		if err == nil {
			s.setExp(false, key, value, 0)
		}
		s.mu.Unlock()

	} else {
		err = ErrNotFound
	}

	return
}

func (s *shardRU) Del(key interface{}) (ok bool) {
	s.mu.Lock()
	ok = s.del(key)
	s.mu.Unlock()
	return
}

func (s *shardRU) del(key interface{}) (ok bool) {

	_, ok = s.payload[key]
	if ok {
		delete(s.payload, key)
		s.counter.Dec()
	}
	return
}

func (s *shardRU) setExp(lock bool, key interface{}, value interface{}, ttl time.Duration) {

	var expire int64
	if ttl == 0 && s.ttl > 0 {
		expire = timeNowLRU(s.ttl)
	}

	cost := s.timer.Tick()
	newItem := &itemLRU{
		Value:  value,
		Expire: expire,
		Cost:   &cost,
	}

	if lock {
		s.mu.Lock()
	}

	s.payload[key] = newItem
	overflow := s.counter.Inc()

	if lock {
		s.mu.Unlock()
	}

	if overflow {
		s.chClean <- struct{}{}
	}
}

func (s *shardRU) GetForRemove(expiredKeys *[]interface{}, oldest iListWithOldEntries) {

	var (
		cost  uint32
		now   = timeNowLRU(0)
		timer = s.timer.Tick()
	)

	s.mu.RLock()
	for k, v := range s.payload {

		if v.Expire <= now {
			notRequireMemoryAllocation := len(*expiredKeys) < cap(*expiredKeys)
			if notRequireMemoryAllocation {
				*expiredKeys = append(*expiredKeys, k)

				if len(*expiredKeys) == cap(*expiredKeys) {
					oldest.Clear() // not all items processed
					break
				}
			}

		} else {
			cost = atomic.LoadUint32(v.Cost)
			oldest.Add(k, cost, timer)
		}
	}
	s.mu.RUnlock()
}

func timeNowLRU(add time.Duration) (v int64) {
	if add == 0 {
		v = time.Now().UnixNano()
	} else {
		v = time.Now().Add(add).UnixNano()
	}
	return
}

type listWithOldEntriesLRU struct {
	keys     []interface{}
	i, limit int
	prevCost uint32
}

func newListWithOldEntriesLRU(limit int) *listWithOldEntriesLRU {
	l := &listWithOldEntriesLRU{limit: limit}
	l.Clear()
	return l
}

func (l *listWithOldEntriesLRU) Add(key interface{}, cost, timer uint32) {

	clear := false

	old := l.i == -1 || (l.prevCost < timer && cost < timer && cost < l.prevCost)
	if !old {
		// previous epoch
		old = cost > timer && l.prevCost < timer
		clear = true // remove from current epoch
	}
	if !old {
		// previous epoch
		old = cost > timer && cost < l.prevCost && l.prevCost > timer
		clear = true // remove from current epoch
	}

	if old {
		if clear {
			l.keys = l.keys[:0]
		}
		if len(l.keys) < cap(l.keys) {
			l.keys = append(l.keys, nil)
		}

		l.i += 1
		if l.i >= len(l.keys) {
			l.i = 0
		}

		l.prevCost = cost
		l.keys[l.i] = key
	}
}

func (l *listWithOldEntriesLRU) Keys() []interface{} {
	return l.keys
}

func (l *listWithOldEntriesLRU) Clear() {
	l.i = -1
	l.keys = make([]interface{}, 0, l.limit)
}
