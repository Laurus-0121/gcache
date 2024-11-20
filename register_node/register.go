package register_node

import (
	"context"
	"fmt"
	clientv3 "go.etcd.io/etcd/client/v3"
	"log"
	"time"
)

var (
	defaultEtcdConfig = clientv3.Config{
		Endpoints:   []string{"localhost:2379"},
		DialTimeout: 5 * time.Second,
	}
)

// AddEtcd在租赁模式下添加一对键值对至etcd
func ectdAddKV(c *clientv3.Client, lid clientv3.LeaseID, service string, addr string) error {
	key := service + "/" + addr
	_, err := c.Put(context.TODO(), key, addr, clientv3.WithLease(lid))
	if err != nil {
		return err
	}
	return nil
}

// Register 注册一个服务至etcd
func Register(service string, addr string, stop chan error) error {
	cli, err := clientv3.New(defaultEtcdConfig)
	if err != nil {
		return fmt.Errorf("create etcd client failed: %v", err)
	}
	defer cli.Close()

	resp, err := cli.Grant(context.Background(), 5)
	if err != nil {
		return fmt.Errorf("create lease failed: %v", err)
	}
	leaseID := resp.ID

	if err := ectdAddKV(cli, leaseID, service, addr); err != nil {
		cli.Revoke(context.Background(), leaseID)
		return fmt.Errorf("add etcd record failed: %v", err)
	}

	ch, err := cli.KeepAlive(context.Background(), leaseID)
	if err != nil {
		cli.Revoke(context.Background(), leaseID)
		return fmt.Errorf("set keepalive failed: %v", err)
	}
	defer cli.Revoke(context.Background(), leaseID)

	log.Printf("[%s] register service ok\n", addr)
	for {
		select {
		case err := <-stop:
			log.Println("Stop signal received: %v\n", err)
			return err
		case <-cli.Ctx().Done():
			log.Println("Service closed")
			return nil
		case ka, ok := <-ch:
			if !ok {
				log.Println("Keep alive channel closed")
				return fmt.Errorf("keep alive channel closed")
			}
			if ka == nil {
				log.Println("Keep alive failed, lease may have expired")
				return fmt.Errorf("keep alive failed")
			}
			log.Printf("Keep alive received,lease ID: %v, TTL:%d\n", ka.ID, ka.TTL)
		}

	}
}
