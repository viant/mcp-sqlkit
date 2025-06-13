package query

import (
    "container/list"
    "reflect"
    "sync"
)

// recordTypeCache provides a concurrency-safe LRU cache for reflect.Type instances
// identified by a string cache key.
// The implementation is intentionally lightweight and does not introduce
// external dependencies – it relies on the standard library list to maintain
// the usage ordering (most-recently-used element at the front).
type recordTypeCache struct {
    mu    sync.Mutex
    ll    *list.List                       // list.Element.Value stores *cacheEntry
    cache map[string]*list.Element
    cap   int
}

type cacheEntry struct {
    key   string
    value reflect.Type
}

// newRecordTypeCache returns an LRU cache with the specified capacity.
// Capacity must be a positive number; otherwise, it defaults to 1.
func newRecordTypeCache(capacity int) *recordTypeCache {
    if capacity <= 0 {
        capacity = 1
    }
    return &recordTypeCache{
        ll:    list.New(),
        cache: make(map[string]*list.Element, capacity),
        cap:   capacity,
    }
}

// Get returns the cached reflect.Type associated with the supplied key, if it
// exists. When found, the entry is promoted to the front of the LRU list,
// marking it as most recently used.
func (c *recordTypeCache) Get(key string) (reflect.Type, bool) {
    c.mu.Lock()
    defer c.mu.Unlock()

    if ele, ok := c.cache[key]; ok {
        c.ll.MoveToFront(ele)
        if ent, _ := ele.Value.(*cacheEntry); ent != nil {
            return ent.value, true
        }
    }
    return nil, false
}

// Put inserts or replaces the reflect.Type associated with key. If the cache
// exceeds its capacity, the least-recently-used entry is evicted.
func (c *recordTypeCache) Put(key string, value reflect.Type) {
    c.mu.Lock()
    defer c.mu.Unlock()

    if ele, ok := c.cache[key]; ok {
        // Update existing element and move it to front
        c.ll.MoveToFront(ele)
        if ent, _ := ele.Value.(*cacheEntry); ent != nil {
            ent.value = value
        }
        return
    }

    // Insert new element at the front
    ele := c.ll.PushFront(&cacheEntry{key: key, value: value})
    c.cache[key] = ele

    // Evict least-recently-used if over capacity
    if c.ll.Len() > c.cap {
        lru := c.ll.Back()
        if lru != nil {
            c.ll.Remove(lru)
            if ent, _ := lru.Value.(*cacheEntry); ent != nil {
                delete(c.cache, ent.key)
            }
        }
    }
}
