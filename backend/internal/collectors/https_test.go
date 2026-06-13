package collectors

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHTTPSCollector_Name(t *testing.T) {
	t.Parallel()
	c := HTTPSCollector{}
	assert.Equal(t, "https", c.Name())
}

func TestHTTPSCollector_ImplementsInterface(t *testing.T) {
	t.Parallel()
	var _ Collector = HTTPSCollector{}
}

func TestHTTPSCollector_NameConsistency(t *testing.T) {
	t.Parallel()
	c := HTTPSCollector{}
	for i := 0; i < 10; i++ {
		assert.Equal(t, "https", c.Name())
	}
}
