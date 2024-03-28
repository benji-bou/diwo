package chantools

import (
	"fmt"
	"io"
	"log/slog"
)

// ChanWriter is really naive implementation of io.Writer to channel
type ChanWriter struct {
	outputC  chan<- []byte
	inputC   chan []byte
	buff     [][]byte
	isClosed bool
}

func (cw *ChanWriter) Write(data []byte) (int, error) {

	if cw.isClosed {
		return 0, fmt.Errorf("underlying channel is closed")
	}
	cw.inputC <- data

	return len(data), nil
}

func (cw *ChanWriter) Close() error {
	cw.isClosed = true
	slog.Debug("close inputC from infiniteImple", "cw.inputC", cw.inputC)
	close(cw.inputC)

	return nil
}

// NewWriter
func NewWriter(c chan<- []byte) io.WriteCloser {
	cw := &ChanWriter{outputC: c, inputC: make(chan []byte), buff: make([][]byte, 0), isClosed: false}
	startInfinitBroker(cw.inputC, cw.outputC)
	return cw
}
