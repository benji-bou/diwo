package diwo

import (
	"fmt"
	"io"
	"log/slog"
)

type Writer struct {
	outputC  chan<- []byte
	inputC   chan []byte
	buff     [][]byte
	isClosed bool
}

func (cw *Writer) Write(data []byte) (int, error) {

	if cw.isClosed {
		return 0, fmt.Errorf("underlying channel is closed")
	}
	cw.inputC <- data

	return len(data), nil
}

func (cw *Writer) Close() error {
	cw.isClosed = true
	slog.Debug("close inputC from infiniteImple", "cw.inputC", cw.inputC)
	close(cw.inputC)

	return nil
}

// NewWriter
func NewWriter(c chan<- []byte) io.WriteCloser {
	cw := &Writer{outputC: c, inputC: make(chan []byte), buff: make([][]byte, 0), isClosed: false}
	startInfinitBroker(cw.inputC, cw.outputC)
	return cw
}
