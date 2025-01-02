package diwo

import (
	"fmt"
	"sync"
	"testing"
)

type brokerTestConfig struct {
	name       string
	srcStarter func()
	src        <-chan int
}

func TestBroker(t *testing.T) {
	starterSrc, src := FromSliceStarter([]int{1, 2, 3, 4, 5, 6, 7, 8, 9})
	emptyStart := func() {
		fmt.Println("start")
	}
	configs := []brokerTestConfig{
		{name: "Nil src", srcStarter: emptyStart, src: nil},
		{name: "Normal src", srcStarter: starterSrc, src: src},
	}

	for _, c := range configs {
		t.Run(c.name, func(t *testing.T) {
			broker := NewBroker(c.src)
			subscriberCount := 10
			subscribers := make([]<-chan int, 0, subscriberCount)
			for range subscriberCount {
				subscribers = append(subscribers, broker.Subscribe())
			}
			c.srcStarter()
			var wg sync.WaitGroup
			for i, subscriber := range subscribers {
				wg.Add(1)
				go func(i int, ch <-chan int) {
					defer wg.Done()
					expected := 1
					for value := range ch {
						if value != expected {
							t.Errorf("Channel %d expected %d, got %v", i, expected, value)
						}
						expected++
					}
				}(i, subscriber)
			}
			wg.Wait()

		})
	}
}

func slicesRange(start int, end int) []int {
	length := end - start
	result := make([]int, length)
	for i := 0; i < length; i++ {
		result[i] = start + i
	}
	return result
}

func TestBrokerSubscribve(t *testing.T) {
	starterSrc, src := FromSliceStarter(slicesRange(0, 4000000))
	b := NewBroker(src)
	subscriberCount := 20
	subscribers := make([]<-chan int, 0, subscriberCount)
	starterSrc()
	for range subscriberCount {
		subscribers = append(subscribers, b.Subscribe())
	}
	if len(subscribers) != len(b.internalBridges) {
		t.Errorf("Expected %d subscribers, got %d", len(subscribers), len(b.internalBridges))
	}
}
