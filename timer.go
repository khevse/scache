package scache

import (
	"sync/atomic"
)

type timer struct {
	value *uint32
}

func newTimer() *timer {
	var value uint32
	return &timer{value: &value}
}

func (t *timer) Tick() (v uint32) {
	v = atomic.AddUint32(t.value, 1)
	return
}

func (t *timer) Value() (v uint32) {
	v = atomic.LoadUint32(t.value)
	return
}
