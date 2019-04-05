package sync

import (
	"fmt"
	"sync"
	"testing"
)

func TestOnceMap_Do(t *testing.T) {
	m := NewOnceMap()

	wg := sync.WaitGroup{}
	var result string
	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func() {
			k := fmt.Sprint(i % 2)
			m.Do(k, func() {
				result += k
			})
			wg.Done()
		}()
	}

	wg.Wait()

	if result != "01" && result != "10" {
		t.Errorf("result is %s", result)
	}
}
