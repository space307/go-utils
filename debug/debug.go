// Copyright 2016 Aleksandr Demakin. All rights reserved.
package debug

import (
	"fmt"
	"net/http"
	_ "net/http/pprof" // for the side effect of registering pakage's HTTP handlers
	"time"

	log "github.com/sirupsen/logrus"
)

var (
	// MinPprofPort the lower limit of the pprof port pool
	MinPprofPort = 1313
	// MaxPprofPort the higher limit of the pprof port pool
	MaxPprofPort = 1353
	// DefaultHost host name for pprof listener
	DefaultHost = ""
)

// StartPprofServer starts pprof server looping over several ports
func StartPprofServer() {
	go func() {
		for {
			for port := MinPprofPort; port <= MaxPprofPort; port++ {
				err := http.ListenAndServe(fmt.Sprintf(DefaultHost+":%d", port), nil)
				log.Errorf("pprof server error: %v", err)
				<-time.After(time.Second)
			}
			<-time.After(time.Second * 5)
		}
	}()
}
