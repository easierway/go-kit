package balancer

import (
	"fmt"
	"testing"
)

func TestNext(t *testing.T) {
	sw := &SmoothWeighted{}
	sw.Add("a", 4)
	sw.Add("b", 2)
	sw.Add("c", 1)
	for i := 0; i < 14; i++ {
		fmt.Println(sw.Next())
	}
}
