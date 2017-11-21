// Copyright 2016 Aleksandr Demakin. All rights reserved.

package sg

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type testServer struct {
	delay    time.Duration
	stopChan chan struct{}
	serveErr error
	stopErr  error
}

func (t *testServer) Serve() error {
	select {
	case <-time.After(t.delay):
	case <-t.stopChan:
	}
	return t.serveErr
}

func (t *testServer) Stop() error {
	close(t.stopChan)
	return t.stopErr
}

func TestSGEmpty(t *testing.T) {
	a := assert.New(t)
	sg := New()
	a.NoError(sg.Serve())
}

func TestSGServeTwice(t *testing.T) {
	a := assert.New(t)
	sg := New()
	a.NoError(sg.Serve())
	a.Error(sg.Serve())
}

func TestSGStopTwice(t *testing.T) {
	a := assert.New(t)
	sg := New()
	a.NoError(sg.Serve())
	a.NoError(sg.Stop())
	a.Error(sg.Stop())
}

func TestSG(t *testing.T) {
	a := assert.New(t)
	sg := New(&testServer{})
	a.Error(sg.Serve())
}

func TestSG2(t *testing.T) {
	a := assert.New(t)
	sg := New(&testServer{delay: time.Second, stopChan: make(chan struct{})})
	go func() {
		for state := range sg.StateChan {
			if state == StateRunning {
				sg.Stop()
			}
		}
	}()
	a.NoError(sg.Serve())
}

func TestSG3(t *testing.T) {
	a := assert.New(t)
	sg := New(&testServer{serveErr: errors.New("err"), delay: time.Second, stopChan: make(chan struct{})})
	go func() {
		for state := range sg.StateChan {
			if state == StateRunning {
				sg.Stop()
			}
		}
	}()
	a.Error(sg.Serve())
}

func TestSGStopError(t *testing.T) {
	a := assert.New(t)
	sg := New(&testServer{stopErr: errors.New("err"), delay: time.Second, stopChan: make(chan struct{})})
	go func() {
		for state := range sg.StateChan {
			if state == StateRunning {
				a.Error(sg.Stop())
			}
		}
	}()
	a.NoError(sg.Serve())
}
