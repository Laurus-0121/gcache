package geecache

// PeerPicker 定义了获取分布式节点的能力
type PeerPicker interface {
	PickPeer(key string) (Fetcher, bool)
}

// Fetcher 定义了从远端获取缓存的能力
// 所以每个Peer应实现这个接口
type Fetcher interface {
	Fetch(group string, key string) (ByteView, error)
}

/*
type ClientPicker struct {
	self        string
	serviceName string
	mu          sync.RWMutex
	consHash    *consistenthash.Map
	clients     map[string]*Client
}

func (p *ClientPicker) PickPeer(key string) (peer PeerGetter, ok bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if peer := p.consHash.Get(key); peer != "" {
		log.Println("Picker peer %s", peer)
		return p.clients[peer], true
	}
	return nil, false
}

func NewClientPicker(self string, opts ...PickerOptions) *ClientPicker {
	picker := ClientPicker{
		self:        self,
		serviceName: defaultServiceName,
		clients:     make(map[string]*Client),
		mu:          sync.RWMutex{},
		consHash:    consistenthash.New(),
	}
	picker.mu.Lock()
	//增量更新
	picker.set(picker.self)
	go func() {
		cli, err := clientv3.New(*register_node.GlobalClientConfig)
		if err != nil {
			log.Fatal(err)
			return
		}
		defer cli.Close()
		watcher := clientv3.NewWatcher(cli)
		watchch := watcher.Watch(context.Background(), picker.serviceName, clientv3.WithPrefix())
		for {
			a := <-watchch
			go func() {
				picker.mu.Lock()
				defer picker.mu.Unlock()
				for _, x := range a.Events {
					key := string(x.Kv.Key)
					idx := strings.Index(key, picker.serviceName)
					addr := key[idx+len(picker.serviceName)+1:]
					if addr == picker.self {
						continue
					}
					if x.IsCreate() {
						if _, ok := picker.clients[addr]; !ok {
							picker.set(addr)
						}
					} else if x.Type == clientv3.EventTypeDelete {
						if _, ok := picker.clients[addr]; ok {
							picker.remove(addr)
						}
					}
				}
			}()
		}
	}()

	//全量更新
	go func() {
		picker.mu.Lock()
		cli, err := clientv3.New(*register_node.GlobalClientConfig)
		if err != nil {
			log.Fatal(err)
			return
		}
		defer cli.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		resp, err := cli.Get(ctx, picker.serviceName, clientv3.WithPrefix())
		if err != nil {
			log.Panic("[Event] full copy request failed")
		}
		kvs := resp.OpResponse().Get().Kvs
		defer picker.mu.Unlock()
		for _, kv := range kvs {
			key := string(kv.Key)
			idx := strings.Index(key, picker.serviceName)
			addr := key[idx+len(picker.serviceName)+1:]

			if _, ok := picker.clients[addr]; !ok {
				picker.set(addr)
			}
		}
	}()

	return &picker

}
func (p *ClientPicker) set(addr string) {
	p.consHash.Register(addr)
	p.clients[addr] = NewClient(addr, p.serviceName)
}
func (p *ClientPicker) remove(addr string) {
	p.consHash.Remove(addr)
	delete(p.clients, addr)
}

type PickerOptions func(picker *ClientPicker)
*/
