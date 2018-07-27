package logrusf

import (
	"bytes"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestFormatter(t *testing.T) {
	b := new(bytes.Buffer)

	logrus.SetOutput(b)
	logrus.SetFormatter(&JSONFormatter{
		Additional: map[string]string{
			"key": "value",
		},
	})

	logrus.
		WithField("test", "test").
		WithField("err", fmt.Errorf("error")).
		Print("message")

	data := struct {
		Time  time.Time
		Msg   string
		Level string
		Key   string
		Test  string
		Err   string
	}{}

	err := json.NewDecoder(b).Decode(&data)
	assert.NoError(t, err)
	assert.Equal(t, "message", data.Msg)
	assert.Equal(t, "info", data.Level)
	assert.Equal(t, "value", data.Key)
	assert.NotEqual(t, time.Time{}, data.Time)
	assert.Equal(t, "test", data.Test)
	assert.Equal(t, "error", data.Err)
}
