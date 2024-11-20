package geecache

import (
	"GeeCache/geecache/lru"
	"sync"
)

// 这样设计可以进行cache和算法的分离，比如我现在实现了lfu缓存模块
// 只需替换cache成员即可

type cache struct {
	mu         sync.RWMutex
	lru        *lru.Cache
	cacheBytes int
}

func newCache(capacity int) *cache {
	return &cache{
		cacheBytes: capacity,
	}
}

func (c *cache) add(key string, value ByteView) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.lru == nil {
		//延迟初始化
		c.lru = lru.New(c.cacheBytes, nil)
	}
	c.lru.Add(key, value)

}

func (c *cache) get(key string) (value ByteView, ok bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.lru == nil {
		return
	}
	if v, ok := c.lru.Get(key); ok {
		return v.(ByteView), ok
	}
	return
}

func (c *cache) remove(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.lru == nil {
		return
	}
	c.lru.Remove(key)
}
