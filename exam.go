package geecache

import (
	"fmt"
	"log"

	"sync"
	"time"
)

func main() {
	// 模拟MySQL数据库 用于cache从数据源获取值
	var mysql = map[string]string{
		"Tom":  "630",
		"Jack": "589",
		"Sam":  "567",
	}
	// 新建cache实例
	group := NewGroup("scores", 2<<10, GetterFunc(
		func(key string) (ByteView, error) {
			log.Println("[Mysql] search key", key)
			if v, ok := mysql[key]; ok {
				return ByteView{[]byte(v), time.Time{}}, nil
			}
			return ByteView{}, fmt.Errorf("%s not exist", key)
		}))

	// New一个服务实例
	var addr = "localhost:9999"
	svr, err := NewServer(addr)
	if err != nil {
		log.Fatal(err)
	}

	// 设置同伴节点IP(包括自己)
	// todo: 这里的peer地址从etcd获取(服务发现)

	svr.SetPeers(addr)

	// 将服务与cache绑定 因为cache和server是解耦合的
	group.RegisterSvr(svr)
	log.Println("gcache is running at", addr)

	// 启动服务(注册服务至etcd/计算一致性哈希...)
	go func() {
		// Start将不会return 除非服务stop或者抛出error
		err = svr.Start()
		if err != nil {
			log.Fatal(err)
		}
	}()

	// 发出几个Get请求
	var wg sync.WaitGroup
	wg.Add(4)
	go GetTomScore(group, &wg)
	go GetTomScore(group, &wg)
	go GetTomScore(group, &wg)
	go GetTomScore(group, &wg)
	wg.Wait()
}

func GetTomScore(group *Group, wg *sync.WaitGroup) {
	defer wg.Done()
	log.Printf("get Tom...")
	view, err := group.Get("Tom")
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	fmt.Println(view.String())
}
