package geecache

import (
	"GeeCache/geecache/singleflight"
	"fmt"
	"log"
	"sync"
	"time"
)

type Getter interface {
	Get(key string) (ByteView, error)
}

// GetterFunc 函数类型实现Getter接口
type GetterFunc func(key string) (ByteView, error)

// Get 通过实现Get方法，使得任意匿名函数func
// 通过被GetterFunc(func)类型强制转换后，实现了 Getter 接口的能力
func (f GetterFunc) Get(key string) (ByteView, error) {
	return f(key)
}

// Group 提供命名管理缓存/填充缓存的能力
type Group struct {
	name      string
	getter    Getter
	mainCache *cache
	hotCache  *cache
	server    PeerPicker
	//use singleflight
	loader           *singleflight.Flight
	emptyKeyDuration time.Duration // getter返回error时对应空值key的过期时间
}

var (
	mu     sync.RWMutex
	groups = make(map[string]*Group)
)

func NewGroup(name string, cacheBytes int, getter Getter) *Group {
	if getter == nil {
		panic("nil Getter")
	}

	mu.Lock()
	defer mu.Unlock()
	g := &Group{
		name:      name,
		getter:    getter,
		mainCache: &cache{cacheBytes: cacheBytes},
		loader:    &singleflight.Flight{},
	}
	groups[name] = g
	return g
}

// SetEmptyWhenError 当getter返回error时设置空值，缓解缓存穿透问题
// 为0表示该机制不生效
func (g *Group) SetEmptyWhenError(duration time.Duration) {
	g.emptyKeyDuration = duration
}

// SetHotCache 设置远程节点Hot Key-Value的缓存，避免频繁请求远程节点
func (g *Group) SetHotCache(cacheBytes int) {
	if cacheBytes <= 0 {
		panic("hot cache must be greater than 0")
	}
	g.hotCache = &cache{
		cacheBytes: cacheBytes,
	}
}

func (g *Group) Get(key string) (ByteView, error) {
	if key == "" {
		return ByteView{}, fmt.Errorf("key is required")
	}

	if v, ok := g.mainCache.get(key); ok { // 先从主缓存获取
		log.Println("[GeeCache] hit")
		return v, nil
	}
	if g.hotCache != nil {
		if v, ok := g.hotCache.get(key); ok { // 主缓存没有看热点缓存
			log.Println("[Cache] hot cache hit")
			return v, nil
		}
	}
	return g.load(key)
}

func (g *Group) Registerserver(server PeerPicker) {
	if g.server != nil {
		panic("RegisterPeerPicker called more than once")
	}
	g.server = server
}

func (g *Group) load(key string) (value ByteView, err error) {
	view, err := g.loader.Fly(key, func() (interface{}, error) {
		if g.server != nil {
			if peer, ok := g.server.PickPeer(key); ok {
				value, err := peer.Fetch(g.name, key)
				if err == nil {
					return value, nil
				}
				log.Printf("fail to get *%s* from peer, %s.\n", key, err.Error())
			}
		}
		return g.getLocally(key)
	})
	if err == nil {
		return view.(ByteView), nil
	}
	return
}

// 从本地获取
func (g *Group) getLocally(key string) (ByteView, error) {
	//1.调用回调函数
	value, err := g.getter.Get(key)
	if err != nil {
		if g.emptyKeyDuration == 0 {
			return ByteView{}, err
		}
		value = ByteView{
			expire: time.Now().Add(g.emptyKeyDuration),
		}
	}
	//2.将源数据添加到缓存mainCache中
	g.populateCache(key, value, g.mainCache)
	return value, nil
}

func (g *Group) populateCache(key string, value ByteView, cache *cache) {
	if cache != nil {
		return
	}
	cache.add(key, value)
}

// 从本地节点删除缓存
func (g *Group) removeLocally(key string) {
	g.mainCache.remove(key)
	if g.hotCache != nil {
		g.hotCache.remove(key)
	}
}

/*func (g *Group) getFromPeer(peer PeerGetter, key string) (ByteView, error) {
	req := &pb.Request{
		Group: g.name,
		Key:   key,
	}
	res := &pb.Response{}
	err := peer.Get(req, res)
	if err != nil {
		return ByteView{}, err
	}
	return ByteView{b: res.Value}, nil
}*/

func (g *Group) RegisterSvr(p PeerPicker) {
	if g.server != nil {
		panic("group had been registered peer")
	}
	g.server = p
}

func GetGroup(name string) *Group {
	mu.RLock()
	g := groups[name]
	mu.RUnlock()
	return g
}

func DestroyGroup(name string) {
	g := GetGroup(name)
	if g != nil {
		svr := g.server.(*server)
		svr.Stop()
		delete(groups, name)
		log.Printf("Destory cache [%s %s]", name, svr.addr)
	}
}
