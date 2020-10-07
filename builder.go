package scache

import (
	"context"
	"errors"
	"math"
	"time"
)

type builder struct {
	conf     *Config
	loadFunc LoadFunc
}

func New(shards int, maxSize int64) *builder {
	return &builder{
		conf: &Config{
			Shards:  shards,
			MaxSize: maxSize,
			Kind:    KindUnknown,
		},
	}
}

func FromConfig(conf *Config) *builder {
	return &builder{
		conf: conf,
	}
}

func (b *builder) LRU() *builder {
	b.conf.Kind = KindLRU
	return b
}

func (b *builder) TTL(val time.Duration) *builder {
	b.conf.TTL = val
	return b
}

func (b *builder) LoaderFunc(val LoadFunc) *builder {
	b.loadFunc = val
	return b
}

func (b *builder) ItemsToPrune(val uint32) *builder {
	b.conf.ItemsToPrune = val
	return b
}

func (b *builder) Build() (*Cache, error) {

	if b.conf.Shards <= 0 || b.conf.Shards >= math.MaxUint32 {
		return nil, errors.New("invalid count of shards")
	}

	if b.conf.MaxSize <= 0 {
		return nil, errors.New("invalid size")
	}

	if b.conf.TTL < 0 {
		return nil, errors.New("invalid cache time to live")
	}

	itemsToPrune := uint32(10)
	if b.conf.ItemsToPrune > 0 {
		itemsToPrune = b.conf.ItemsToPrune
	}

	shards := make([]iShard, 0, int(b.conf.Shards))
	counter := newCounter(b.conf.MaxSize)
	timer := newTimer()

	cleanLimit := 1000
	if v := int(counter.Limit()) / 10; v > cleanLimit {
		cleanLimit = v
	}
	chClean := make(chan struct{}, cleanLimit)

	for i := 0; i < b.conf.Shards; i++ {
		var shard iShard
		switch b.conf.Kind {
		case KindLRU:

			shard = newShardRU(chClean, counter, timer, b.conf, b.loadFunc)
		default:
			return nil, errors.New("invalid kind of cache")
		}

		shards = append(shards, shard)
	}

	ctx, ctxCancel := context.WithCancel(context.Background())
	c := &Cache{
		maxShardIndex: uint64(len(shards)) - 1,
		shards:        shards,
		counter:       counter,
		itemsToPrune:  itemsToPrune,
		ctx:           ctx,
		ctxCancel:     ctxCancel,
		chClean:       chClean,
		timer:         timer,
	}
	c.runCleaner()

	return c, nil
}
