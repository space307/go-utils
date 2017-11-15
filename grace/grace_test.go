package grace

import (
	"syscall"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

// Test that basic signal handling works.
func TestOnRestart(t *testing.T) {
	sighup := false
	OnRestart(func() {
		sighup = true
	})

	// Send this process a SIGHUP
	err := syscall.Kill(syscall.Getpid(), syscall.SIGHUP)
	if err != nil {
		log.Errorf("error ")
	}
	time.Sleep(time.Second)
	require.Equal(t, true, sighup)
}

func TestOnShutdown(t *testing.T) {
	sigterm := false
	OnShutdown(func() {
		sigterm = true
	})

	// Send this process a SIGHUP
	err := syscall.Kill(syscall.Getpid(), syscall.SIGTERM)
	if err != nil {
		log.Errorf("error ")
	}
	time.Sleep(time.Second)
	require.Equal(t, true, sigterm)
}
