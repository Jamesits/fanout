package main

import (
	"github.com/hashicorp/nomad/api"
	"github.com/spf13/pflag"
	"net"
	"os"
	"sync"
)

var (
	service       string
	listenAddress string
	allowStale    bool

	mu sync.RWMutex
	sg []*api.ServiceRegistration
)

func init() {
	pflag.StringVar(&service, "service", "", "service to connect to")
	pflag.StringVar(&listenAddress, "listen-address", "[::1]:8080", "listener address")
	pflag.BoolVar(&allowStale, "allow-stale", true, "allow reading stale values from Nomad API")
	pflag.Parse()
}

func main() {
	if service == "" {
		errorLogger.Error("service name is empty, refusing")
		os.Exit(255)
	}

	// start Nomad connection
	go updateWorkloadIdentity()
	go updateServices()

	// start listener
	lServer, err := net.ResolveTCPAddr("tcp", listenAddress)
	if err != nil {
		errorLogger.Error("unable to resolve listen address", "address", listenAddress, "error", err)
		os.Exit(255)
	}
	listener, err := net.ListenTCP("tcp", lServer)
	if err != nil {
		errorLogger.Error("unable to listen", "address", lServer, "error", err)
		os.Exit(255)
	}

	// TCP connection handling
	for {
		_ = handleTCPConn(listener)
	}
}
