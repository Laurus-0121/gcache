package geecache

import (
	"fmt"
	"log"
	"math/rand"
	"reflect"
	"testing"
	"time"
)

func createTestSvr() (*Group, *server) {
	mysql := map[string]string{
		"Tom":  "630",
		"Jack": "589",
		"Sam":  "567",
	}

	g := NewGroup("scores", 2<<10, GetterFunc(
		func(key string) (ByteView, error) {
			log.Println("[Mysql] search key", key)
			if v, ok := mysql[key]; ok {
				return ByteView{b: []byte(v)}, nil
			}
			return ByteView{}, fmt.Errorf("%s not exist", key)
		}))

	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	port := 50000 + r.Intn(100)
	addr := fmt.Sprintf("localhost: %d", port)

	svr, err := NewServer(addr)
	if err != nil {
		log.Fatal(err)
	}
	svr.SetPeers(addr)
	g.RegisterSvr(svr)
	return g, svr
}

func TestServer_GetExistKey(t *testing.T) {
	g, svr := createTestSvr()
	go func() {
		err := svr.Start()
		if err != nil {
			log.Fatal(err)
		}
	}()

	view, err := g.Get("Tom")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(view.String(), "630") {
		t.Errorf("Tom %s(actual)/%s(ok)", view.String(), "630")
	}
	log.Printf("Tom -> %s", view.String())
	DestroyGroup(g.name)
}
