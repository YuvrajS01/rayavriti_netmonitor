package logging

import (
	"io"
	"os"
)

// MultiSink writes log output to multiple destinations simultaneously.
type MultiSink struct {
	writers []io.Writer
}

// NewMultiSink creates a sink that fans out to all provided writers.
func NewMultiSink(writers ...io.Writer) *MultiSink {
	return &MultiSink{writers: writers}
}

// Write implements io.Writer — writes to all sinks; returns on first error.
func (m *MultiSink) Write(p []byte) (n int, err error) {
	for _, w := range m.writers {
		n, err = w.Write(p)
		if err != nil {
			return
		}
	}
	return
}

// StdoutSink returns an io.Writer that writes to stdout.
func StdoutSink() io.Writer { return os.Stdout }

// StderrSink returns an io.Writer that writes to stderr.
func StderrSink() io.Writer { return os.Stderr }
