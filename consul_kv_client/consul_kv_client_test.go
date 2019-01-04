package consul_kv_client

import "testing"

func TestNewConsulKVClient(t *testing.T) {
	client, err := NewConsulKVClient("127.0.0.1:8500")
	if err != nil {
		panic(err)
	}

	key := "test"
	val := []byte("hello")
	if err := client.Put(key, val); err != nil {
		panic(err)
	}
	res, err := client.Get(key)
	if err != nil {
		panic(err)
	}
	if len(res) == 0 {
		t.Fatal("ConsulKVClient Get error")
	}
}
