package lru

import (
	"GeeCache/geecache/zset"
	"container/list"
	"time"
)

// 缓存算法对比：
// LRU：最近最少使用。它很综合，如果数据最近很少被使用，那么就会被淘汰。它的实现很简单，使用map+双向链表。
// LFU：最不经常使用。它根据访问次数来决定是否被淘汰，可能会存在某个一段时间很热的key在另外一段时间不那么热，却由于积累的访问次数过大而无法被淘汰。它的实现使用两个map+双向链表。https://juejin.cn/post/6987260805888606245#heading-2

// Warning: lru包不提供并发一致机制
// TODO: 实现lru-k
const (
	expiresZSetKey = ""
	// 每次移除过期键数量
	removeExpireN = 10
)

type Value interface {
	Len() int
	Expire() time.Time
}
type entry struct {
	key   string
	value Value
}

type Cache struct {
	maxBytes  int        //最大内存
	nbytes    int        //已使用的内存
	ll        *list.List //双向链表
	cache     map[string]*list.Element
	onEvicted func(key string, value Value) //某条记录被移除时的回调函数
	expires   *zset.SortedSet
}

func New(maxBytes int, onEvicted func(string, Value)) *Cache {
	return &Cache{
		maxBytes:  maxBytes,
		ll:        list.New(),
		cache:     make(map[string]*list.Element),
		onEvicted: onEvicted,
		expires:   zset.New(),
	}
}
func (c *Cache) Get(key string) (value Value, ok bool) {
	element, ok := c.cache[key]
	if !ok {
		return nil, false
	}
	ent := element.Value.(*entry)
	// 移除过期的键
	if !ent.value.Expire().IsZero() && ent.value.Expire().Before(time.Now()) {
		c.removeElement(element)
		return nil, false
	}
	c.ll.MoveToBack(element)
	return ent.value, true
}

// 移除指定键，并删除链表里面的节点，减少lru缓存大小，删除过期时间，调用回调函数
func (c *Cache) removeElement(e *list.Element) {
	c.ll.Remove(e)
	kv := e.Value.(*entry)
	delete(c.cache, kv.key)
	c.nbytes -= len(kv.key) + kv.value.Len()
	// 移除过期键
	if !kv.value.Expire().IsZero() {
		c.expires.ZRem(expiresZSetKey, kv.key)
	}
	if c.onEvicted != nil {
		c.onEvicted(kv.key, kv.value)
	}
}

// 移除最近最少访问的节点
// 缓存淘汰
func (c *Cache) RemoveOldest() {
	ele := c.ll.Back()
	if ele != nil {
		c.ll.Remove(ele)
		kv := ele.Value.(*entry)
		delete(c.cache, kv.key)
		c.nbytes -= int(len(kv.key)) + int(kv.value.Len())
		if c.onEvicted != nil {
			c.onEvicted(kv.key, kv.value)
		}
	}
}

func (c *Cache) Add(key string, value Value) {
	if element, ok := c.cache[key]; ok {
		c.ll.MoveToBack(element)
		ent := element.Value.(*entry)
		c.nbytes += value.Len() - ent.value.Len()
		ent.value = value
	} else {
		ent := &entry{
			key:   key,
			value: value,
		}
		element := c.ll.PushBack(ent)
		c.cache[key] = element
		c.nbytes += len(key) + value.Len()
	}
	// 如果有超时时间则设置
	if !value.Expire().IsZero() {
		c.expires.ZAdd(expiresZSetKey, value.Expire().UnixNano(), key)
	} else {
		// 没有则删除
		c.expires.ZRem(expiresZSetKey, key)
	}
	// 淘汰过期的key
	if c.maxBytes != 0 {
		c.removeExpire(removeExpireN)
	}

	// 淘汰最近最少访问的key
	for c.maxBytes != 0 && c.nbytes > c.maxBytes {
		c.RemoveOldest()
	}
}

// 移除过期的键
// 返回未删除的数量
func (c *Cache) removeExpire(n int) int {
	for n > 0 && c.expires.ZCard(expiresZSetKey) > 0 {
		values := c.expires.ZRangeWithScores(expiresZSetKey, 0, 0)
		key, expireNano := values[0].(string), values[1].(int64)
		// 第一个键都没超时，结束循环
		if expireNano > time.Now().UnixNano() {
			break
		}
		c.Remove(key)
		n--
	}
	return n
}

// Remove 移除某个键
func (c *Cache) Remove(key string) {
	if element, ok := c.cache[key]; ok {
		c.removeElement(element)
	}
}
func (c *Cache) Len() int {
	return c.ll.Len()
}
