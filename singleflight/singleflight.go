package singleflight

import "sync"

// single flight 为cache提供缓存击穿的保护
// 当cache并发访问peer获取缓存时 如果peer未缓存该值
// 则会向db发送大量的请求获取 造成db的压力骤增
// 因此 将所有由key产生的请求抽象成flight
// 这个flight只会起飞一次(single) 这样就可以缓解击穿的可能性
// flight载有我们要的缓存数据 称为packet

type call struct {
	wg  sync.WaitGroup
	val interface{}
	err error
}

type Flight struct {
	mu sync.Mutex
	m  map[string]*call
}

func (g *Flight) Fly(key string, fn func() (interface{}, error)) (interface{}, error) {
	g.mu.Lock()
	if g.m == nil {
		g.m = make(map[string]*call)
	}
	if c, ok := g.m[key]; ok {
		g.mu.Unlock()
		c.wg.Wait()
		return c.val, c.err
	}
	c := new(call)
	c.wg.Add(1)
	g.m[key] = c
	g.mu.Unlock()

	c.val, c.err = fn()
	c.wg.Done()

	g.mu.Lock()
	delete(g.m, key) //更新g.m
	g.mu.Unlock()

	return c.val, c.err
}
