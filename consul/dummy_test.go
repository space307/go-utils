package consul

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDummyConsulWrapper(t *testing.T) {
	client := NewDummyClient()
	assert.Implements(t, (*TTLUpdater)(nil), client)

	assert.NoError(t, client.UpdateTTL("", "", ""))
}
