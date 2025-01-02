package diwo

import (
	"slices"
	"testing"
)

type testBasic[T any] func(t *testing.T, obtained T, expected []int)

func equalArTest(t *testing.T, obtained []int, expected []int) {
	if !slices.Equal(expected, obtained) {
		t.Errorf("received values and obtained value differ")
		return
	}
}

type unionTestConfig[T any] struct {
	unionFunc func(c ...<-chan int) T
	expected  []int
	testing   testBasic[T]
	chQty     int
}

func testChanRead(testIntegrity func(t *testing.T, expected []int, obtained []int)) testBasic[<-chan int] {
	return func(t *testing.T, ch <-chan int, expected []int) {
		t.Helper()
		if ch == nil {
			t.Errorf("Have nil channel. It will block forever")
			return
		}
		obtained := make([]int, 0, len(expected))
		for i := range ch {
			if slices.Contains(obtained, i) {
				t.Errorf("received value duplication. Value sent for testing are unique. You should not receive it twice: %d", i)
				return
			}
			if slices.Contains(expected, i) {
				obtained = append(obtained, i)
			} else {
				t.Errorf("received value not expected. got %d is not in expected array of value", i)
				return
			}
		}
		testIntegrity(t, expected, obtained)

	}
}

func testingUnionOperator[T any](t *testing.T, config unionTestConfig[T]) {
	t.Helper()
	t.Run("With empty array", func(t *testing.T) {
		ch := config.unionFunc()
		config.testing(t, ch, []int{})
	})
	t.Run("With nil array", func(t *testing.T) {
		var nilAr []<-chan int = nil
		ch := config.unionFunc(nilAr...)
		config.testing(t, ch, []int{})
	})
	t.Run("With nil chan", func(t *testing.T) {
		ch := config.unionFunc(nil)
		config.testing(t, ch, []int{})
	})
	t.Run("With mix nil and concret chan", func(t *testing.T) {
		ch := config.unionFunc(FromSlice(config.expected[0:len(config.expected)/2]), nil, FromSlice(config.expected[len(config.expected)/2:]), nil)
		config.testing(t, ch, config.expected)
	})
	t.Run("normal behavior", func(t *testing.T) {
		chs := make([]<-chan int, 0, config.chQty)
		for i := 0; i < config.chQty; i++ {
			indexStart := i * len(config.expected) / config.chQty
			indexStop := min((i+1)*len(config.expected)/config.chQty, len(config.expected))
			chs = append(chs, FromSlice(config.expected[indexStart:indexStop]))
		}
		ch := config.unionFunc(chs...)
		config.testing(t, ch, config.expected)

	})
}

func TestMerge(t *testing.T) {
	expected := []int{1, 2, 3, 4, 5, 6, 7, 8, 9}
	config := unionTestConfig[<-chan int]{
		chQty:     3,
		unionFunc: Merge[int],
		expected:  expected,
		testing: testChanRead(func(t *testing.T, expected []int, obtained []int) {
			t.Helper()
			slices.Sort(obtained)
			equalArTest(t, obtained, expected)
		}),
	}
	testingUnionOperator(t, config)
}

func TestCollect(t *testing.T) {

	type collectorTestConfig struct {
		name        string
		collectorFn CollectorFunc[int]
		testing     testBasic[[]int]
	}
	collectorToTest := [](collectorTestConfig){
		{
			name:        "Collect Using Merge",
			collectorFn: Collect[int](Merge),
			testing: func(t *testing.T, obtained []int, expected []int) {
				slices.Sort(obtained)
				equalArTest(t, obtained, expected)
			},
		},
		{
			name:        "Collect Using Concat",
			collectorFn: Collect[int](Concat),
			testing:     equalArTest,
		},
	}
	expected := []int{1, 2, 3, 4, 5, 6}
	config := unionTestConfig[[]int]{
		chQty:    3,
		expected: expected,
	}
	for _, ct := range collectorToTest {
		t.Run(ct.name, func(t *testing.T) {
			config.unionFunc = ct.collectorFn
			config.testing = ct.testing
			testingUnionOperator(t, config)
		})
	}

}

func TestConcat(t *testing.T) {
	expected := []int{1, 2, 3, 4, 5, 6}
	config := unionTestConfig[<-chan int]{
		chQty:     3,
		unionFunc: Concat[int],
		expected:  expected,
		testing: testChanRead(func(t *testing.T, expected, obtained []int) {
			if !slices.Equal(expected, obtained) {
				t.Errorf("did not reveiveves expected values in correct order. got %v, want %v", obtained, expected)
			}
		}),
	}
	testingUnionOperator(t, config)
}

func TestMap(t *testing.T) {
	ch := FromSlice([]int{1, 2, 3})
	mapped := Map(ch, func(input int) int { return input * 2 })

	expected := []int{2, 4, 6}
	for _, e := range expected {
		value, ok := <-mapped
		if !ok || value != e {
			t.Errorf("Expected %d, got %v", e, value)
		}
	}
}

func TestForwardTo(t *testing.T) {
	src := FromSlice([]int{1, 2, 3})
	dst := make(chan int)
	go ForwardTo(src, dst)

	expected := []int{1, 2, 3}
	for _, e := range expected {
		value, ok := <-dst
		if !ok || value != e {
			t.Errorf("Expected %d, got %v", e, value)
		}
	}

	if _, ok := <-dst; ok {
		t.Error("Expected dst to be closed")
	}
}

func TestFilter(t *testing.T) {
	ch := FromSlice([]int{1, 2, 3, 4, 5})
	filtered := Filter(ch, func(element int) bool { return element%2 == 0 })

	expected := []int{2, 4}
	for _, e := range expected {
		value, ok := <-filtered
		if !ok || value != e {
			t.Errorf("Expected %d, got %v", e, value)
		}
	}
}

func TestMapSlice(t *testing.T) {
	input := []int{1, 2, 3}
	result := mapSlice(input, func(input int) int { return input * 2 })

	expected := []int{2, 4, 6}
	if len(result) != len(expected) {
		t.Errorf("Expected length %d, got %d", len(expected), len(result))
	}
	for i, e := range expected {
		if result[i] != e {
			t.Errorf("At index %d, expected %d, got %v", i, e, result[i])
		}
	}
}
