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
		now            = timeNowLRU(0)
		expiredKeysLen = len(*expiredKeys)
		expiredKeysCap = cap(*expiredKeys)
	)

	s.mu.RLock()
	for k, v := range s.payload {

		if v.Expire <= now {
			if expiredKeysLen < expiredKeysCap {
				*expiredKeys = append(*expiredKeys, k)
				expiredKeysLen++
			}

		} else {
			oldest.Add(k, v.Cost, s.timer.Value())
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

type itemListOldestLRU struct {
	Key interface{}
	// Cost and SaveCost must be equal before operation delete
	Cost     *uint32
	SaveCost uint32
}

func (i *itemListOldestLRU) Valid() (ok bool) {

	if i.Cost != nil {
		cost := atomic.LoadUint32(i.Cost)
		if cost == i.SaveCost {
			ok = true
		}
	}

	return
}

func (i *itemListOldestLRU) Clear() {
	i.Key = nil
	i.Cost = nil
	i.SaveCost = 0
}

type listWithOldEntriesLRU struct {
	beforeMaxCost *epoch
	afterMaxCost  *epoch
}

func newListWithOldEntriesLRU(limit int) *listWithOldEntriesLRU {
	return &listWithOldEntriesLRU{
		beforeMaxCost: newEpoch(limit),
		afterMaxCost:  newEpoch(limit),
	}
}

func (l *listWithOldEntriesLRU) Add(key interface{}, itemCost *uint32, maxCost uint32) {

	cost := atomic.LoadUint32(itemCost)

	if cost < maxCost {
		// current epoch
		l.beforeMaxCost.AddItem(key, itemCost, cost)

	} else if cost > maxCost {
		// previous epoch
		l.afterMaxCost.AddItem(key, itemCost, cost)
	}
}

func (l *listWithOldEntriesLRU) Next() (key interface{}, ok bool) {

	key, ok = l.afterMaxCost.Next()
	if !ok {
		key, ok = l.beforeMaxCost.Next()
	}

	return
}

func (l *listWithOldEntriesLRU) Clear() {
	l.beforeMaxCost.Clear()
	l.afterMaxCost.Clear()
}

type epoch struct {
	items   []*itemListOldestLRU
	lastIdx int
	readed  int
}

func newEpoch(limit int) *epoch {

	e := &epoch{
		items: make([]*itemListOldestLRU, limit),
	}

	for i := range e.items {
		e.items[i] = &itemListOldestLRU{}
	}

	return e
}

func (e *epoch) AddItem(key interface{}, cost *uint32, saveCost uint32) {

	const EmptyIdx = -1
	var (
		idx        = EmptyIdx
		first      = e.items[0]
		maxIdx     = len(e.items) - 1
		prev, item *itemListOldestLRU
	)

	if first.Cost == nil || first.SaveCost > saveCost {
		idx = 0

	} else if maxIdx > 0 && first.Cost != nil && first.SaveCost < saveCost {

		if e.lastIdx == 0 && e.lastIdx < maxIdx {
			idx = e.lastIdx + 1

		} else {
			found := false
			mid, left, right := 0, 0, e.lastIdx

			for left <= right {
				mid = (right + left) / 2
				item = e.items[mid]
				if item.SaveCost > saveCost {
					if mid > 0 {
						if prev = e.items[mid-1]; prev.SaveCost < saveCost {
							idx = mid
							found = true
							break
						}
					}
					right = mid - 1
				} else if item.SaveCost == saveCost {
					idx = EmptyIdx
					break
				} else {
					left = mid + 1
				}
			}

			if !found && e.lastIdx < maxIdx {
				idx = e.lastIdx + 1
			}

		}
	}

	if idx == EmptyIdx {
		return
	}

	lastIdx := e.lastIdx
	if first.Cost != nil && lastIdx < maxIdx {
		e.lastIdx++
		lastIdx = e.lastIdx
	}

	// the worst situation: O(N*ItemsToPrune)
	for i := lastIdx; i > idx; i-- {
		prev = e.items[i-1]
		item = e.items[i]
		item.Key = prev.Key
		item.Cost = prev.Cost
		item.SaveCost = prev.SaveCost
	}

	item = e.items[idx]
	item.Key = key
	item.Cost = cost
	item.SaveCost = saveCost
}

func (e *epoch) Clear() {
	for _, item := range e.items {
		item.Clear()
	}
}

func (e *epoch) Next() (key interface{}, ok bool) {

	var item *itemListOldestLRU

	for e.readed < len(e.items) {
		item = e.items[e.readed]
		e.readed++

		if item.Valid() {
			key = item.Key
			ok = true
			return
		}
	}

	return
}
