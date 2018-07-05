package checker

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVerify(t *testing.T) {
	err := Verify([]Checkup{})
	assert.NoError(t, err)

	err = Verify([]Checkup{func() error {
		return fmt.Errorf("some error")
	}})
	assert.EqualError(t, err, "some error")
}
