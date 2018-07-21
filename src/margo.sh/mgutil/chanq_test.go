package mgutil

import (
	"fmt"
	"testing"
)

func TestChanQ(t *testing.T) {
	for _, i := range []int{0, -1} {
		name := fmt.Sprintf("NewChanQ(%d)", i)
		t.Run(name, func(t *testing.T) {
			defer func() {
				if v := recover(); v == nil {
					t.Errorf("%s does not result in a panic", name)
				}
			}()
			NewChanQ(i)
		})
	}

	cq := NewChanQ(1)
	lastVal := -1
	for i := 0; i < 3; i++ {
		lastVal = i
		cq.Put(lastVal)
	}
	if v := <-cq.C(); v != lastVal {
		t.Error("CtxQ.Put does not appear to clear the old value")
	}
}
