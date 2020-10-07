package scache

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestBuilder(t *testing.T) {

	for _, i := range []int{-1, 0, int(math.MaxUint32), int(math.MaxUint32) + 1} {
		c, err := New(i, 0).Build()
		require.EqualError(t, err, "invalid count of shards")
		require.Nil(t, c)
	}

	for _, i := range []int64{-1, 0} {
		c, err := New(1, i).Build()
		require.EqualError(t, err, "invalid size")
		require.Nil(t, c)
	}

	for _, i := range []time.Duration{-1} {
		c, err := New(1, 1).TTL(i).Build()
		require.EqualError(t, err, "invalid cache time to live")
		require.Nil(t, c)
	}

	{
		c, err := New(1, 1).Build()
		require.EqualError(t, err, "invalid kind of cache")
		require.Nil(t, c)
	}

	c, err := New(1, 1).LRU().Build()
	require.NoError(t, err)
	require.NotNil(t, c)
}
