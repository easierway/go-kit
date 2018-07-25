package balancer

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestConsulResolver(t *testing.T) {
	r, err := NewConsulResolver("127.0.0.1:8500", "hatlonly-test-service", "my-service", 200*time.Millisecond)
	if err != nil {
		panic(err)
	}
	defer r.Stop()
	counter := map[string]int{}
	N := 10000
	for i := 0; i < N; i++ {
		address := r.DiscoverNode().Address
		if _, ok := counter[address]; !ok {
			counter[address] = 0
		}
		counter[address]++
	}
	for key, val := range counter {
		fmt.Printf("%v => %v%%\n", key, float64(val)*100.0/float64(N))
	}
}

func TestConcurrency(t *testing.T) {
	r, err := NewConsulResolver("127.0.0.1:8500", "hatlonly-test-service", "my-service", 200*time.Millisecond)
	if err != nil {
		panic(err)
	}
	defer r.Stop()

	var wg sync.WaitGroup
	now := time.Now()
	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func() {
			counter := map[string]int{}
			num := 0
			for {
				if time.Since(now) > time.Duration(20)*time.Second {
					break
				}
				address := r.DiscoverNode().Address
				if _, ok := counter[address]; !ok {
					counter[address] = 0
				}
				counter[address]++
				num++
			}
			wg.Done()
		}()
	}
	wg.Wait()
}
