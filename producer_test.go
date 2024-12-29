package diwo

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
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
				t.Errorf("Error Basik work stream output. Want %s, Got %s at Index %d", streamStr[o.Index], o.Value, o.Index)
			}
		}
	})
}

func TestNewWithDefaultOptions(t *testing.T) {
	result := New(func(c chan<- int) {
		c <- 42
	})

	value, ok := <-result
	if !ok || value != 42 {
		t.Errorf("Expected 42, got %v", value)
	}
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

func TestNewWithName(t *testing.T) {
	name := uuid.NewString()
	_ = New(func(c chan<- int) {}, WithName(name))
	// No errors should occur
}

func TestFromSlice(t *testing.T) {
	slice := []int{1, 2, 3, 4, 5}
	result := FromSlice(slice)

	for i, expected := range slice {
		value, ok := <-result
		if !ok || value != expected {
			t.Errorf("At index %d, expected %d, got %v", i, expected, value)
		}
	}
}

func TestBroadcast(t *testing.T) {
	src := FromSlice([]int{1, 2, 3})
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
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	counter := 0
	result := Tick(ctx, 10*time.Millisecond, func(t time.Time) int {
		counter++
		return counter
	})

	expected := 1
	for value := range result {
		if value != expected {
			t.Errorf("Expected %d, got %v", expected, value)
		}
		expected++
	}
}

func TestInfiniteWrapper(t *testing.T) {
	workerChan := make(chan int, 3)
	inputChan := infiniteWrapper(workerChan)

	inputChan <- 1
	inputChan <- 2
	close(inputChan)

	expected := []int{1, 2}
	for _, e := range expected {
		value, ok := <-workerChan
		if !ok || value != e {
			t.Errorf("Expected %d, got %v", e, value)
		}
	}

	if _, ok := <-workerChan; ok {
		t.Error("Expected workerChan to be closed")
	}
}

func TestStartInfinitBroker(t *testing.T) {
	inputChan := make(chan int, 3)
	outputChan := make(chan int, 3)

	go startInfinitBroker(inputChan, outputChan)

	inputChan <- 1
	inputChan <- 2
	close(inputChan)

	expected := []int{1, 2}
	for _, e := range expected {
		value, ok := <-outputChan
		if !ok || value != e {
			t.Errorf("Expected %d, got %v", e, value)
		}
	}

	if _, ok := <-outputChan; ok {
		t.Error("Expected outputChan to be closed")
	}
}

func TestEdgeCases(t *testing.T) {
	// Empty slice for FromSlice
	result := FromSlice([]int{})
	if _, ok := <-result; ok {
		t.Error("Expected channel to be closed for empty slice")
	}

	// Nil tearDown function
	_ = New(func(c chan<- int) {}, WithTearDown(nil))
	// No errors should occur
}
