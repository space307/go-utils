// Copyright 2016 Aleksandr Demakin. All rights reserved.
package grace

import (
	"os"
	"os/signal"
	"syscall"

	log "github.com/sirupsen/logrus"
)

// OnShutdown executes given func on SIGINT or SIGTERM.
func OnShutdown(f func()) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		s := <-ch
		log.Infof("got a signal: %v", s.String())
		f()
	}()
}

// OnRestart executes given func on SIGHUP.
func OnRestart(f func()) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGHUP)
	go func() {
		s := <-ch
		log.Infof("got a signal: %v", s.String())
		f()
	}()
}
