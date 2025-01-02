package diwo

import (
	"fmt"
	"slices"
	"testing"
	"time"
)

func TestNewPool(t *testing.T) {
	initialVector := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20}
	src := FromSlice(initialVector)
	pool := NewPool(3, src, func(elem int, c chan<- int) {
		c <- elem * 2
	})

	if len(pool) != 3 {
		t.Errorf("Expected pool size to be 3, got %d", len(pool))
		return
	}
	res := CollectMerge(pool...)
	if len(res) != len(initialVector) {
		t.Errorf("Expected result length to be %d, got %d", len(initialVector), len(res))
		return
	}
	slices.Sort(res)
	res = slices.Compact(res)
	if len(res) != len(initialVector) {
		t.Errorf("Duplicated results expected result length to be %d, got %d", len(initialVector), len(res))
		return
	}
	for i, value := range res {
		if value/2 != initialVector[i] {
			t.Errorf("Expected even number, got %d want %d", value, initialVector[i])
		}
	}
}

func TestNumberOfWorkerPool(t *testing.T) {
	initialVector := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20}
	src := FromSlice(initialVector)
	timeStartWorker := time.Now()
	pool := NewPool(3, src, func(elem int, c chan<- time.Duration) {
		now := time.Now()
		time.Sleep(10 * time.Millisecond)
		c <- time.Since(now)
	})
	res := CollectMerge(pool...)
	workerPoolDuration := time.Since(timeStartWorker)
	eachWorkerDurationTotal := time.Duration(0)
	for _, d := range res {
		eachWorkerDurationTotal += d
	}
	fmt.Printf("Sum of each worker took %v\n", eachWorkerDurationTotal)
	fmt.Printf("Total pool took %v\n", workerPoolDuration)
	if eachWorkerDurationTotal > workerPoolDuration*3 {
		t.Errorf("Sum of each worker took more than 3 times the total time")
	}
}
