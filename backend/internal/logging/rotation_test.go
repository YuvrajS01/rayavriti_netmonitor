package logging

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewRotatingWriter(t *testing.T) {
	t.Parallel()
	w := NewRotatingWriter(RotationConfig{
		Filename:   "/tmp/test.log",
		MaxSizeMB:  50,
		MaxBackups: 5,
		MaxAgeDays: 7,
		Compress:   true,
	})
	assert.NotNil(t, w)
}

func TestNewRotatingWriter_Defaults(t *testing.T) {
	t.Parallel()
	w := NewRotatingWriter(RotationConfig{Filename: "/tmp/test.log"})
	assert.NotNil(t, w)
}
