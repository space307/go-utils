// Copyright 2017 Aleksandr Demakin. All rights reserved.

package sg

import (
	"fmt"
	"sync/atomic"

	"github.com/pkg/errors"
)

var (
	// ServerGroup itself must satifsy the Server interface.
	_ Server = (*ServerGroup)(nil)
)

const (
	// StateIdle is a state for a new group.
	StateIdle = 0
	// StateStarting is a state after a call to Serve()
	StateStarting = 1
	// StateRunning is a state when all servers were started.
	StateRunning = 2
	// StateStopping is a state after a call to Stop().
	StateStopping = 3
	// StateStopped is a state after all servers were stopped.
	StateStopped = 4
)

// Server is an object, which starts some processing in Serve.
// Processing can be stopped by calling Stop.
type Server interface {
	Serve() error
	Stop() error
}

// ServerGroup is a group of servers running simultaneously.
// They start working by calling StartAll and can be stopped by calling StopAll.
// If one of the servers stops working, StopAll is called automatically.
type ServerGroup struct {
	// StateChan is a chan for group's state changes.
	StateChan chan int32
	servers   []serverInfo
	errChan   chan serverRunResult
	resChan   chan error
	state     int32
}

type serverInfo struct {
	s        Server
	stopped  int32
	critical bool
}

type serverRunResult struct {
	err error
	id  int
}

// New creates a new ServerGroup with all servers critical.
func New(servers ...Server) *ServerGroup {
	result := &ServerGroup{state: StateIdle, StateChan: make(chan int32, 4)}
	for _, s := range servers {
		result.Add(s, true)
	}
	return result
}

// Add adds a server and allows to specify 'critical' flag.
//	if true, the group will be stopped after server's failure.
//	if false, the group continues running.
// Add must be called before Serve.
func (sg *ServerGroup) Add(s Server, critical bool) {
	sg.servers = append(sg.servers, serverInfo{s: s, critical: critical})
}

// Serve starts all the servers.
func (sg *ServerGroup) Serve() error {
	if !atomic.CompareAndSwapInt32(&sg.state, StateIdle, StateStarting) {
		return fmt.Errorf("invalid group state. already started?")
	}
	sg.reportState(StateStarting)
	sg.errChan = make(chan serverRunResult, len(sg.servers))
	sg.resChan = make(chan error)
	for idx, si := range sg.servers {
		go func(s Server, idx int) {
			sg.errChan <- serverRunResult{err: s.Serve(), id: idx}
		}(si.s, idx)
	}
	go sg.waitAll()
	sg.reportState(StateRunning)
	atomic.StoreInt32(&sg.state, StateRunning)
	return <-sg.resChan
}

// Stop stops all the servers.
func (sg *ServerGroup) Stop() error {
	var result error
	if !atomic.CompareAndSwapInt32(&sg.state, StateRunning, StateStopping) {
		return fmt.Errorf("group isn't in running state. already stopped, or wasn't started?")
	}
	sg.reportState(StateStopping)
	for i := range sg.servers {
		if !atomic.CompareAndSwapInt32(&sg.servers[i].stopped, 0, 1) {
			continue
		}
		if err := sg.servers[i].s.Stop(); err != nil {
			result = errors.Wrap(err, "server error")
		}
	}
	atomic.StoreInt32(&sg.state, StateStopped)
	sg.reportState(StateStopped)
	return result
}

func (sg *ServerGroup) waitAll() {
	var result error
	for range sg.servers {
		runRes := <-sg.errChan
		if atomic.CompareAndSwapInt32(&sg.servers[runRes.id].stopped, 0, 1) { // the server stopped itself.
			if result == nil {
				result = errors.New("the server stopped prematurely")
			}
		}
		if runRes.err != nil {
			result = runRes.err
		}
		// Stop checks if the servers are still running, and if they are, stops them all.
		if sg.servers[runRes.id].critical {
			sg.Stop()
		}
	}
	sg.resChan <- result
}

func (sg *ServerGroup) reportState(state int32) {
	select {
	case sg.StateChan <- state:
	default:
	}
}
