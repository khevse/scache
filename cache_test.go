package scache

import (
	"errors"
	"strconv"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCache(t *testing.T) {

	for _, testInfo := range []struct {
		Name string
		Func func(Kind) func(*testing.T)
	}{
		{
			Name: "GetSet",
			Func: func(kind Kind) func(*testing.T) {
				return func(*testing.T) {
					testCacheSetAndGet(t, kind)
				}
			},
		},
		{
			Name: "LoadFunc",
			Func: func(kind Kind) func(*testing.T) {
				return func(*testing.T) {
					testCacheLoadFunc(t, kind)
				}
			},
		},
	} {

		for _, kind := range []Kind{KindLRU} {
			if !t.Run(testInfo.Name, testInfo.Func(kind)) {
				return
			}
		}
	}
}

func testCacheLoadFunc(t *testing.T, kind Kind) {

	conf := &Config{
		Shards:  2,
		MaxSize: 4,
		Kind:    kind,
	}

	loadFunc := func(key interface{}) (val interface{}, err error) {
		if key == "1" {
			val, err = "1", nil
		} else {
			err = errors.New("failed to upload")
		}
		return
	}

	cache, err := FromConfig(conf).LoaderFunc(loadFunc).Build()
	require.NoError(t, err)

	for i := 0; i < 3; i++ {
		{
			val, err := cache.Get(strconv.Itoa(1))
			require.NoError(t, err)
			require.Equal(t, "1", val)
		}

		{
			val, err := cache.Get(strconv.Itoa(2))
			require.EqualError(t, err, "failed to upload")
			require.Nil(t, val)
		}
	}

}

func testCacheSetAndGet(t *testing.T, kind Kind) {

	conf := Config{
		Shards:  2,
		MaxSize: 4,
		Kind:    kind,
	}
	cache, err := FromConfig(&conf).Build()
	require.NoError(t, err)

	keys := []string{"a", "b", "c", "d"}

	for i, k := range keys {
		cache.Set(k, i)
		val, err := cache.Get(k)
		require.NoError(t, err)
		require.Equal(t, i, val)
	}

	for i, b := range cache.shards {
		require.Greater(t, b.Count(), int64(0), i)
	}
}

func BenchmarkBaseSCache(b *testing.B) {

	countOverflowKeys := 101

	size := b.N
	if size > countOverflowKeys {
		size /= 3
		size -= countOverflowKeys
	}

	keysCount := size + countOverflowKeys

	keys := make([]interface{}, keysCount)
	for i := 0; i < keysCount; i++ {
		keys[i] = "------" + strconv.Itoa(i) // "----" - check hash function for shard id
	}

	loadFunc := func(key interface{}) (value interface{}, err error) {
		value = key
		return
	}

	cache, err := New(100, int64(size)).ItemsToPrune(20).LRU().LoaderFunc(loadFunc).Build()
	require.NoError(b, err)
	defer cache.Close()

	var (
		hits  int64
		pHits = &hits
	)

	b.ResetTimer()
	b.StartTimer()

	for i := 0; i < b.N; i++ {
		op := i % 3
		key := keys[i%len(keys)]

		switch op {
		case 1:
			cache.Del(key)
		default:
			_, err = cache.Get(key)
			if err == nil {
				atomic.AddInt64(pHits, 1)
			}
		}
	}

	b.StopTimer()

	if hits < int64(b.N/2) {
		b.Error("hits", hits, "b.N", b.N)
	}
}
