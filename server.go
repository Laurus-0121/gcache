package geecache

import (
	"GeeCache/geecache/consistenthash"
	pb "GeeCache/geecache/geecachepb"
	"GeeCache/geecache/register_node"
	"context"
	"fmt"
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc"
	"log"
	"net"
	"strings"
	"sync"
	"time"
)

const (
	defaultAddr        = "127.0.0.1:6324"
	defaultReplicas    = 50
	defaultServiceName = "LaurusCache"
)

var (
	defaultEtcdConfig = clientv3.Config{
		Endpoints:   []string{"localhost:2379"},
		DialTimeout: 5 * time.Second,
	}
)

type server struct {
	pb.UnimplementedGroupCacheServer

	addr       string
	status     bool
	stopSignal chan error
	mu         sync.Mutex
	consHash   *consistenthash.Map
	clients    map[string]*Client
}

/*
	func (s *server) SetPeers(peerAddr ...string) {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.consHash = consistenthash.New(defalutReplicas, nil)
		s.consHash.Register(peerAddr...)
		s.clients = make(map[string]*Client)
		for _, peerAddr := range peerAddr {
			if !validPeerAddr(peerAddr) {
				panic(fmt.Errorf("[peer %s] invalid address format, it should be x.x.x.x:prot", peerAddr))
			}
			service := fmt.Sprintf("geecache/%s", peerAddr)
			s.clients[peerAddr] = NewClient(service)

		}

}
*/
func (s *server) PickPeer(key string) (Fetcher, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	peerAddr := s.consHash.Get(key)
	if peerAddr == s.addr {
		log.Printf("pick local peer: %s\n", s.addr)
		return nil, false
	}
	log.Printf("[cache %s] pick remote peer: %s\n", s.addr, peerAddr)
	return s.clients[peerAddr], true
}

// NewServer 创建cache的server 若addr为空，则使用defaultAddr
func NewServer(addr string) (*server, error) {
	if addr == "" {
		addr = defaultAddr
	}
	if !validPeerAddr(addr) {
		return nil, fmt.Errorf("invalid addr %s", addr)
	}
	return &server{addr: addr}, nil
}

func (s *server) Get(ctx context.Context, in *pb.Request) (*pb.Response, error) {
	group, key := in.GetGroup(), in.GetKey()
	resp := &pb.Response{}

	log.Printf("[geecache_server %s] Recv RPC Request - (%s)/(%s)", s.addr, group, key)
	if key == "" {
		return resp, fmt.Errorf("key require")
	}
	g := GetGroup(group)
	if g == nil {
		return resp, fmt.Errorf("group not found")
	}
	view, err := g.Get(key)
	if err != nil {
		return resp, err
	}
	resp.Value = view.ByteSlice()
	return resp, nil

}

func (s *server) Start() error {
	s.mu.Lock()

	if s.status == true {
		s.mu.Unlock()
		return fmt.Errorf("server already start")
	}
	// -----------------启动服务----------------------
	// 1. 设置status为true 表示服务器已在运行
	// 2. 初始化stop channal,这用于通知registry stop keep alive
	// 3. 初始化tcp socket并开始监听
	// 4. 注册rpc服务至grpc 这样grpc收到request可以分发给server处理
	// 5. 将自己的服务名/Host地址注册至etcd 这样client可以通过etcd
	//    获取服务Host地址 从而进行通信。这样的好处是client只需知道服务名
	//    以及etcd的Host即可获取对应服务IP 无需写死至client代码中
	// ----------------------------------------------
	s.status = true
	s.stopSignal = make(chan error)

	port := strings.Split(s.addr, ":")[1]
	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		return fmt.Errorf("failed to listen: %v", err)
	}
	grpcServer := grpc.NewServer()
	pb.RegisterGroupCacheServer(grpcServer, s)

	go func() {
		err := register_node.Register("LaurusCache", s.addr, s.stopSignal)
		if err != nil {
			log.Fatalf(err.Error())
		}
		close(s.stopSignal)
		err = lis.Close()
		if err != nil {
			log.Fatalf(err.Error())
		}
		log.Printf("[%s] Revoke service and close tcp socket ok.", s.addr)
	}()
	s.mu.Unlock()
	if err := grpcServer.Serve(lis); s.status && err != nil {
		return fmt.Errorf("failed to serve: %v", err)
	}
	return nil
}
func (s *server) Stop() {
	s.mu.Lock()
	if s.status == false {
		s.mu.Unlock()
		return
	}
	s.stopSignal <- nil
	s.status = false
	s.clients = nil //清空一致性哈希 有助于垃圾回收
	s.consHash = nil
	s.mu.Unlock()
}

// SetPeers 将各个远端主机IP配置到Server里
// 这样Server就可以Pick他们了
// 注意: 此操作是*覆写*操作！
// 注意: peersIP必须满足 x.x.x.x:port的格式
func (s *server) SetPeers(peersAddr ...string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.consHash = consistenthash.New(defaultReplicas, nil)
	s.consHash.Register(peersAddr...)
	s.clients = make(map[string]*Client)
	for _, peerAddr := range peersAddr {
		if !validPeerAddr(peerAddr) {
			panic(fmt.Sprintf("[peer %s] invalid address format, it should be x.x.x.x:port", peerAddr))
		}
		service := fmt.Sprintf("gcache/%s", peerAddr)
		s.clients[peerAddr] = NewClient(service)
	}
}

// Pick 根据一致性哈希选举出key应存放在的cache
// return false 代表从本地获取cache
func (s *server) Pick(key string) (Fetcher, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	peerAddr := s.consHash.Get(key)
	// Pick itself
	if peerAddr == s.addr {
		log.Printf("ooh! pick myself, I am %s\n", s.addr)
		return nil, false
	}
	log.Printf("[cache %s] pick remote peer: %s\n", s.addr, peerAddr)
	return s.clients[peerAddr], true
}

// 测试Server是否实现了Picker接口
var _ PeerPicker = (*server)(nil)
