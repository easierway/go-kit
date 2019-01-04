package consul_kv_client

import (
	"github.com/hashicorp/consul/api"
)

type ConsulKVClient struct {
	client *api.Client
}

func NewConsulKVClient(address string) (*ConsulKVClient, error) {
	config := api.DefaultConfig()
	config.Address = address
	client, err := api.NewClient(config)
	if err != nil {
		return nil, err
	}
	return &ConsulKVClient{client: client}, nil
}

func (c *ConsulKVClient) Get(key string) ([]byte, error) {
	res, _, err := c.client.KV().Get(key, nil)
	if err != nil {
		return nil, err
	}
	return res.Value, nil
}

func (c *ConsulKVClient) Put(key string, val []byte) error {
	_, err := c.client.KV().Put(&api.KVPair{Key: key, Value: val}, nil)
	if err != nil {
		return err
	}
	return nil
}
