package scache

import (
	"time"
)

type Config struct {
	Kind         Kind
	TTL          time.Duration
	MaxSize      int64
	Shards       int
	ItemsToPrune uint32
}
