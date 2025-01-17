package diwo

import (
	"context"
	"sync"
	"testing"
	"time"
)

type ValueIndexed struct {
	Index int
	Value string
}

func TestNew(t *testing.T) {
	streamStr := []string{"elem1", "elem2", "elem3", "elem4", "elem5"}

	t.Run("basic work", func(t *testing.T) {

		outputC := New(func(c chan<- ValueIndexed) {
			for i, s := range streamStr {
				c <- ValueIndexed{Index: i, Value: s}
			}
		})
		for o := range outputC {
			if streamStr[o.Index] != o.Value {
				t.Errorf("Error Basic work stream output. Want %s, Got %s at Index %d", streamStr[o.Index], o.Value, o.Index)
			}
		}
	})
}

func TestNewWithTearDown(t *testing.T) {
	tearDownCalled := false
	result := New(func(c chan<- int) {
		c <- 42
	}, WithTearDown(func() { tearDownCalled = true }))

	value, ok := <-result
	if !ok || value != 42 {
		t.Errorf("Expected 42, got %v", value)
	}

	// Allow worker to complete
	time.Sleep(10 * time.Millisecond)
	if !tearDownCalled {
		t.Error("TearDown function was not called")
	}
}

func TestFromSlice(t *testing.T) {
	slice := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13}
	result := FromSlice(slice)

	for i, expected := range slice {
		value, ok := <-result
		if !ok || value != expected {
			t.Errorf("At index %d, expected %d, got %v", i, expected, value)
		}
	}
}

func TestBroadcast(t *testing.T) {
	src := FromSlice([]int{1, 2, 3, 4, 5, 6, 7, 8, 9})
	channels := Broadcast(src, 3)

	var wg sync.WaitGroup
	for i, ch := range channels {
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
		}(i, ch)
	}
	wg.Wait()
}

func TestTick(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 70*time.Millisecond)
	defer cancel()

	counter := time.Now()
	result := Tick(ctx, 10*time.Millisecond, func(t time.Time) time.Duration {
		d := t.Sub(counter)
		counter = t
		return d
	})

	expected := 10 * time.Millisecond
	for value := range result {
		if value > expected-5 && value < expected+5 {
			t.Errorf("Expected %d, got %v", expected, value)
		}
	}
}

func TestInfiniteWrapper(t *testing.T) {
	count := 100000
	outputC := make(chan int)
	inputInfiniteChan := infiniteWrapper(outputC)

	for i := 0; i < count; i++ {
		inputInfiniteChan <- i

	}
	close(inputInfiniteChan)
	i := 0
	for value := range outputC {
		if i >= count {
			t.Errorf("Received to much value")
		}
		if value != i {
			t.Errorf("Expected %d, got %v", i, value)
		}
		i++
	}
	if i == 0 {
		t.Errorf("did not read any data from outputC")
	}

	if _, ok := <-outputC; ok {
		t.Error("Expected outputC to be closed")
	}
}

func TestOnce(t *testing.T) {
	tC := Once(42)
	i := <-tC
	if i != 42 {
		t.Errorf("Not the correct value sent")
	}
	i, ok := <-tC
	if ok {
		t.Errorf("channel once should be closed, received %d", i)
	}
}

func TestUnmangedChan(t *testing.T) {
	count := 20
	workerFunc := func(c chan<- int) {
		for i := 1; i < count; i++ {
			c <- i
		}
	}

	testingFunc := func(t *testing.T, c <-chan int, testClosed func()) {
		i := 1
		var tC <-chan time.Time = nil

	L:
		for {
			select {
			case <-tC:
				break L
			case tData, ok := <-c:
				if ok == false {
					testClosed()
					break L
				}
				if tData != i {
					t.Errorf("incorrect data want %d, got %v", i, tData)
					return
				}
				if i == count-1 {
					tC = time.Tick(3 * time.Second)
				}
				i++
			}
		}
		if i != count {
			t.Errorf("did not received correct number of data got %d want %d ", i, count-1)
			return
		}
	}

	t.Run("Nomal New With Unmanaged should still closed", func(t *testing.T) {
		c := New(workerFunc, WithUnmanaged())
		testingFunc(t, c, func() {
		})
	})

	t.Run("NewWithChanBuilder and Unmanaged should not close", func(t *testing.T) {
		c := NewWithChan(make(chan int), workerFunc, WithUnmanaged())

		testingFunc(t, c, func() {
			t.Error("chan should not be closed")
		})
	})

}

func TestEmpty(t *testing.T) {
	emptyCh := Empty[int]()
	_, ok := <-emptyCh
	if ok {
		t.Errorf("empty channel is not directly closed")
	}

}
