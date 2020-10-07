package scache

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLruSetAndGet(t *testing.T) {

	cache := newShardRU(nil, newCounter(1000), newTimer(), &Config{
		TTL: 1 * time.Second,
	}, nil)

	const Key = "test"

	cache.Set(Key, "TEST DATA1")
	cache.Set(Key, "TEST DATA2")

	val, err := cache.Get(Key)
	require.NoError(t, err)
	require.Equal(t, "TEST DATA2", val)
}

func TestLruOldestList(t *testing.T) {

	type InData struct {
		Key         string
		Cost, Timer uint32
	}

	for i, testInfo := range []struct {
		Limit  int
		Res    []interface{}
		InData []InData
	}{
		{
			// one epoch. old is first
			Limit: 1,
			Res:   []interface{}{"1"},
			InData: []InData{
				{Key: "1", Cost: 1, Timer: 3},
				{Key: "2", Cost: 2, Timer: 3},
				{Key: "3", Cost: 3, Timer: 3},
			},
		},
		{
			// one epoch. old is in the middle
			Limit: 1,
			Res:   []interface{}{"5"},
			InData: []InData{
				{Key: "4", Cost: 2, Timer: 3},
				{Key: "5", Cost: 1, Timer: 3},
				{Key: "6", Cost: 3, Timer: 3},
			},
		},
		{
			// one epoch. old is last
			Limit: 1,
			Res:   []interface{}{"9"},
			InData: []InData{
				{Key: "7", Cost: 2, Timer: 3},
				{Key: "8", Cost: 3, Timer: 3},
				{Key: "9", Cost: 1, Timer: 3},
			},
		},
		{
			// two epoch. old is first
			Limit: 1,
			Res:   []interface{}{"10"},
			InData: []InData{
				{Key: "10", Cost: 10, Timer: 3},
				{Key: "11", Cost: 1, Timer: 3},
				{Key: "12", Cost: 3, Timer: 3},
			},
		},
		{
			// two epoch. old is in the middle
			Limit: 1,
			Res:   []interface{}{"14"},
			InData: []InData{
				{Key: "13", Cost: 1, Timer: 3},
				{Key: "14", Cost: 10, Timer: 3},
				{Key: "15", Cost: 3, Timer: 3},
			},
		},
		{
			// two epoch. old is last
			Limit: 1,
			Res:   []interface{}{"18"},
			InData: []InData{
				{Key: "16", Cost: 1, Timer: 3},
				{Key: "17", Cost: 3, Timer: 3},
				{Key: "18", Cost: 10, Timer: 3},
			},
		},
		{
			// two epoch. old is in the middle
			Limit: 1,
			Res:   []interface{}{"20"},
			InData: []InData{
				{Key: "19", Cost: 1, Timer: 3},
				{Key: "20", Cost: 10, Timer: 3},
				{Key: "21", Cost: 11, Timer: 3},
			},
		},
		{
			// two epoch. old is in the middle
			Limit: 3,
			Res:   []interface{}{"23"},
			InData: []InData{
				{Key: "22", Cost: 1, Timer: 3},
				{Key: "23", Cost: 10, Timer: 3},
				{Key: "25", Cost: 11, Timer: 3},
				{Key: "27", Cost: 2, Timer: 3},
				{Key: "26", Cost: 12, Timer: 3},
			},
		},
	} {
		l := newListWithOldEntriesLRU(testInfo.Limit)
		for _, item := range testInfo.InData {
			l.Add(item.Key, item.Cost, item.Timer)
		}

		require.Equal(t, testInfo.Res, l.keys, i)
		l.Clear()
	}

}

func TestLruTTL(t *testing.T) {

	chClean := make(chan struct{}, 10)
	cache := newShardRU(chClean, newCounter(2), newTimer(), &Config{
		TTL: 1 * time.Second,
	}, nil)

	cache.Set("test1", "test1")
	cache.Set("test2", "test2")
	for _, k := range []string{"test1", "test2"} {
		v, err := cache.Get(k)
		require.NoError(t, err)
		require.Equal(t, k, v)
	}

	cache.Set("test3", "test3")
	{
		v, err := cache.Get("test3")
		require.NoError(t, err)
		require.Equal(t, "test3", v)
	}

	time.Sleep(cache.ttl + time.Millisecond/2)

	for _, k := range []string{"test1", "test2"} {
		v, err := cache.Get(k)
		require.Equal(t, ErrNotFound, err)
		require.Nil(t, v)
	}

	_, ok := <-chClean
	require.True(t, ok) // remove test1 before add test3

	select {
	case _, ok = <-chClean:
		require.False(t, ok)
		t.Fatal("not empty")
	default:
		// ok
	}

}

func TestLruGetUnsetted(t *testing.T) {

	cache := newShardRU(nil, newCounter(1000), newTimer(), &Config{
		TTL:          1 * time.Second,
		ItemsToPrune: 1,
	}, nil)

	v, err := cache.Get("test")
	require.Equal(t, ErrNotFound, err)
	require.Nil(t, v)
}

func TestLruDel(t *testing.T) {

	cache := newShardRU(nil, newCounter(2), newTimer(), &Config{
		TTL:          1 * time.Second,
		ItemsToPrune: 1,
	}, nil)

	cache.Set("key", "DATA")
	cache.Del("key")

	v, err := cache.Get("test")
	require.Equal(t, ErrNotFound, err)
	require.Nil(t, v)
}
