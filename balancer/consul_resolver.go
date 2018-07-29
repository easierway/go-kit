package balancer

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net"
	"sort"
	"strconv"
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/shirou/gopsutil/cpu"
)

// ConsulResolverBuilder builder
type ConsulResolverBuilder struct {
	Address   string
	Service   string
	Interval  time.Duration
	MyService string
	Ratio     float64
}

// Build a ConsulResolver
func (b *ConsulResolverBuilder) Build() (*ConsulResolver, error) {
	return NewConsulResolver(
		b.Address, b.Service, b.MyService, b.Interval, b.Ratio,
	)
}

// NewConsulResolver create a new ConsulResolver
func NewConsulResolver(
	address string, service string, myService string, interval time.Duration, ratio float64,
) (*ConsulResolver, error) {
	config := api.DefaultConfig()
	config.Address = address
	client, err := api.NewClient(config)
	if err != nil {
		return nil, err
	}

	r := &ConsulResolver{
		client:    client,
		service:   service,
		myService: myService,
		interval:  interval,
		ratio:     ratio,
		done:      false,
		cpuUsage:  50,
		zone:      zone(),
	}

	if err := r.Start(); err != nil {
		return nil, err
	}

	fmt.Printf("new consul resolver %#v\n", r)

	return r, nil
}

// ConsulResolver consul resolver
type ConsulResolver struct {
	client          *api.Client
	service         string
	lastIndex       uint64
	myService       string
	myLastIndex     uint64
	zone            string
	factorThreshold int
	myServiceNum    int
	localZone       *ServiceZone
	otherZone       *ServiceZone
	interval        time.Duration
	done            bool
	cpuUsage        int
	ratio           float64
}

// ServiceNode service node
type ServiceNode struct {
	Address       string
	Host          string
	Port          int
	Zone          string
	BalanceFactor int
}

// ServiceZone service zone
type ServiceZone struct {
	Nodes     []*ServiceNode
	Factors   []int
	FactorMax int
}

// Start resolve
func (r *ConsulResolver) Start() error {
	if err := r.updateCPUUsage(); err != nil {
		return err
	}
	if err := r.updateFactorThreshold(); err != nil {
		return err
	}
	if err := r.updateServiceZone(); err != nil {
		return err
	}

	go func() {
		for range time.Tick(r.interval) {
			if r.done {
				break
			}
			r.updateCPUUsage()
		}
	}()
	go func() {
		for range time.Tick(r.interval) {
			if r.done {
				break
			}
			r.updateFactorThreshold()
		}
	}()
	go func() {
		for range time.Tick(r.interval) {
			if r.done {
				break
			}
			r.updateServiceZone()
		}
	}()

	return nil
}

// Stop resolve
func (r *ConsulResolver) Stop() {
	r.done = true
}

// GetLocalZone local zone
func (r *ConsulResolver) GetLocalZone() *ServiceZone {
	return r.localZone
}

// GetOtherZone local zone
func (r *ConsulResolver) GetOtherZone() *ServiceZone {
	return r.otherZone
}

// DiscoverNode get one address
func (r *ConsulResolver) DiscoverNode() *ServiceNode {
	localZone := r.localZone
	otherZone := r.otherZone

	if localZone.FactorMax+otherZone.FactorMax == 0 {
		return nil
	}

	factorThreshold := r.factorThreshold
	if r.ratio != 0 {
		m := float64((localZone.FactorMax+otherZone.FactorMax)*r.myServiceNum) * r.ratio
		n := float64(len(localZone.Factors) + len(otherZone.Factors))
		factorThreshold = int(m / n)
	}
	factorThreshold = factorThreshold * r.cpuUsage / 100

	serviceZone := localZone
	if factorThreshold > localZone.FactorMax || localZone.FactorMax <= 0 {
		factorMax := otherZone.FactorMax + localZone.FactorMax
		if factorMax > factorThreshold && factorThreshold > 0 {
			factorMax = factorThreshold
		}
		factor := rand.Intn(factorMax)
		if factor >= localZone.FactorMax {
			serviceZone = otherZone
		}
	}
	idx := sort.SearchInts(serviceZone.Factors, rand.Intn(serviceZone.FactorMax))
	return serviceZone.Nodes[idx]
}

func (r *ConsulResolver) updateCPUUsage() error {
	percentage, err := cpu.Percent(0, true)
	if err != nil {
		return err
	}
	p := int(percentage[0])
	if p <= 0 {
		r.cpuUsage = 1
	} else {
		r.cpuUsage = p
	}

	return nil
}

func (r *ConsulResolver) updateFactorThreshold() error {
	services, metainfo, err := r.client.Health().Service(r.myService, "", true, &api.QueryOptions{
		WaitIndex: r.myLastIndex,
	})
	if err != nil {
		return fmt.Errorf("error retrieving instances from Consul: %v", err)
	}
	r.myLastIndex = metainfo.LastIndex

	factorThreshold := 0
	myServiceNum := 0
	for _, service := range services {
		balanceFactor := 0
		zone := "unknown"
		if balanceFactorStr, ok := service.Service.Meta["balanceFactor"]; ok {
			if i, err := strconv.Atoi(balanceFactorStr); err == nil {
				balanceFactor = i
			}
		}
		if z, ok := service.Service.Meta["zone"]; ok {
			zone = z
		}
		if zone == r.zone {
			factorThreshold += balanceFactor
			myServiceNum++
		}
	}

	r.factorThreshold = factorThreshold
	r.myServiceNum = myServiceNum
	fmt.Printf("update factorThreshold [%v], lastIndex [%v]\n", factorThreshold, r.myLastIndex)
	fmt.Printf("update myServiceNum [%v], lastIndex [%v]\n", myServiceNum, r.myLastIndex)

	return nil
}

func (r *ConsulResolver) updateServiceZone() error {
	services, metainfo, err := r.client.Health().Service(r.service, "", true, &api.QueryOptions{
		WaitIndex: r.lastIndex,
	})
	if err != nil {
		return fmt.Errorf("error retrieving instances from Consul: %v", err)
	}
	r.lastIndex = metainfo.LastIndex

	var localZone ServiceZone
	var otherZone ServiceZone

	for _, service := range services {
		balanceFactor := 100
		zone := "unknown"
		if balanceFactorStr, ok := service.Service.Meta["balanceFactor"]; ok {
			if i, err := strconv.Atoi(balanceFactorStr); err == nil {
				balanceFactor = i
			}
		}
		if z, ok := service.Service.Meta["zone"]; ok {
			zone = z
		}
		node := &ServiceNode{
			Address:       net.JoinHostPort(service.Service.Address, strconv.Itoa(service.Service.Port)),
			Host:          service.Service.Address,
			Port:          service.Service.Port,
			BalanceFactor: balanceFactor,
			Zone:          zone,
		}
		if zone == r.zone {
			localZone.Nodes = append(localZone.Nodes, node)
			localZone.FactorMax += node.BalanceFactor
			localZone.Factors = append(localZone.Factors, localZone.FactorMax)
		} else {
			otherZone.Nodes = append(otherZone.Nodes, node)
			otherZone.FactorMax += node.BalanceFactor
			otherZone.Factors = append(otherZone.Factors, otherZone.FactorMax)
		}
	}

	r.localZone = &localZone
	r.otherZone = &otherZone

	var buf []byte
	buf, _ = json.Marshal(localZone)
	fmt.Printf("update localZone [%v], lastIndex [%v]\n", string(buf), r.lastIndex)
	buf, _ = json.Marshal(otherZone)
	fmt.Printf("update otherZone [%v], lastIndex [%v]\n", string(buf), r.lastIndex)

	return nil
}
