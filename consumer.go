package diwo

import "iter"

func Seq[C any](input <-chan C) iter.Seq[C] {
	return func(yield func(C) bool) {
		for i := range input {
			if !yield(i) {
				return
			}
		}
	}
}
