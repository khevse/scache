package scache

import (
	"sync/atomic"
)

type counter struct {
	limit int64
	val   int64
}

func newCounter(limit int64) *counter {
	return &counter{
		limit: limit,
	}
}

func (c *counter) Inc() (overflow bool) {
	overflow = c.limit < atomic.AddInt64(&c.val, 1)
	return
}

func (c *counter) Limit() (val int64) {
	val = c.limit
	return
}

func (c *counter) Dec() {
	atomic.AddInt64(&c.val, -1)
}

func (c *counter) Count() int64 {
	return atomic.LoadInt64(&c.val)
}
