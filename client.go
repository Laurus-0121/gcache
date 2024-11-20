package geecache

import (
	pb "GeeCache/geecache/geecachepb"
	"GeeCache/geecache/register_node"
	"context"
	"fmt"
	clientv3 "go.etcd.io/etcd/client/v3"
	"time"
)

type Client struct {
	name string
}

func (c *Client) Fetch(group string, key string) (ByteView, error) {
	cli, err := clientv3.New(defaultEtcdConfig)
	if err != nil {
		return ByteView{}, err
	}
	defer cli.Close()
	conn, err := register_node.EtcdDial(cli, c.name)
	if err != nil {
		return ByteView{}, err
	}
	defer conn.Close()
	grpcClient := pb.NewGroupCacheClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	resp, err := grpcClient.Get(ctx, &pb.Request{
		Group: group,
		Key:   key,
	})
	if err != nil {
		return ByteView{}, fmt.Errorf("could not get %s/%s from peer %s", group, key, c.name)
	}
	var expire time.Time
	if resp.Expire != 0 {
		expire = time.Unix(resp.Expire/int64(time.Second), resp.Expire%int64(time.Second))
		if time.Now().After(expire) {
			return ByteView{}, fmt.Errorf("peer returned expired value")
		}
	}

	return ByteView{resp.Value, expire}, nil
}

/*
	func (c *Client)Delete(group string, key string) ([]byte, error) {
		cli, err := clientv3.New(defaultEtcdConfig)
		if err != nil {
			return nil, err
		}
		defer cli.Close()
		conn, err := register_node.EtcdDial(cli, c.name)
		if err != nil {
			return nil, err
		}
		defer conn.Close()
		grpcClient := pb.NewGroupCacheClient(conn)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		resp, err := grpcClient.Delete(ctx, &pb.Request{
			Group: group,
			Key:   key,
		})
		if err != nil {
			return nil, fmt.Errorf("could not get %s/%s from peer %s", group, key, c.name)
		}
		return resp.GetValue(), nil
	}
*/
func NewClient(addr string) *Client {
	return &Client{name: addr}

}

var _ Fetcher = (*Client)(nil)
