package chantools

import (
	"io"
)

// ChanWriter is really naive implementation of io.Writer to channel
type ChanWriter chan<- []byte

func (cw ChanWriter) Write(data []byte) (int, error) {
	cw <- data
	return len(data), nil
}

func NewWriter(c chan<- []byte) io.Writer {
	return ChanWriter(c)
}
