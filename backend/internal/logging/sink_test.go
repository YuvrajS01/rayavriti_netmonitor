package logging

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMultiSink_Write_AllWriters(t *testing.T) {
	t.Parallel()
	var written1, written2, written3 []byte
	w1 := &mockWriter{data: &written1}
	w2 := &mockWriter{data: &written2}
	w3 := &mockWriter{data: &written3}

	sink := NewMultiSink(w1, w2, w3)
	n, err := sink.Write([]byte("hello"))
	assert.NoError(t, err)
	assert.Equal(t, 5, n)
	assert.Equal(t, []byte("hello"), written1)
	assert.Equal(t, []byte("hello"), written2)
	assert.Equal(t, []byte("hello"), written3)
}

func TestMultiSink_Write_OneErrors(t *testing.T) {
	t.Parallel()
	w1 := &mockWriter{data: &[]byte{}}
	w2 := &mockWriter{err: assert.AnError, data: &[]byte{}}
	w3 := &mockWriter{data: &[]byte{}}

	sink := NewMultiSink(w1, w2, w3)
	_, err := sink.Write([]byte("test"))
	assert.Error(t, err)
}

type mockWriter struct {
	data *[]byte
	err  error
}

func (m *mockWriter) Write(p []byte) (int, error) {
	if m.err != nil {
		return 0, m.err
	}
	*m.data = append(*m.data, p...)
	return len(p), nil
}
