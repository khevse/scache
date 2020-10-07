# scache

Cache library for golang. It supports expirable cache: LRU

## Install

go get github.com/khevse/scache

## Examples

Simple:
```bash
c, err := scache.New(100, 10000).TTL(time.Hour).LRU().Build()
```

With loader function:
```bash
loadFunc = func(key interface{}) (value interface{}, err error) {
    ... load from DB
    value = rowData
    return
}

c, err := scache.New(10, 10000).LRU().LoaderFunc(loadFunc).Build()
```

From configuration:
```bash
conf := &scache.Config{
    Shards:       2,
    MaxSize:      4,
    Kind:         scache.scache.KindLRU,
    TTL:          time.Hour,
    ItemsToPrune: 10,
}

loadFunc := func(key interface{}) (val interface{}, err error) {
    val = key
    return
}

c, err := scache.FromConfig(conf).LoaderFunc(loadFunc).Build()
```

## Benchmarks

The benchmarks can be found in [page](https://github.com/khevse/cachebenchmarks)