package scache

import (
	"errors"
	"strconv"
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
