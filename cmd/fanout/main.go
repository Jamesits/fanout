package main

import (
	"github.com/hashicorp/nomad/api"
	"github.com/spf13/pflag"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

var (
	service       string
	listenAddress string
	allowStale    bool

	mu        sync.RWMutex
	sg        []*api.ServiceRegistration
	closeOnce sync.Once
	connWG    sync.WaitGroup
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
	go handleTCPConn(listener)
    errorLogger.Info("Fanout started", "service", service, "listen_address", listenAddress)

	// wait for exit
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGQUIT, syscall.SIGTERM, syscall.SIGINT)
	for {
		s := <-c

		// https://eli.thegreenplace.net/2020/graceful-shutdown-of-a-tcp-server-in-go/
		closeOnce.Do(func() {
			errorLogger.Info("stopping the listener")
			err = listener.Close()
			if err != nil {
				errorLogger.Error("unable to stop the listener", "error", err)
			}
		})

		switch s {
		case syscall.SIGTERM:
			errorLogger.Warn("force exiting")
			os.Exit(0)
		case syscall.SIGQUIT, syscall.SIGINT:
			errorLogger.Info("graceful exit triggered, waiting for current connections to finish")
			go func() {
				connWG.Wait()
				os.Exit(0)
			}()
		default:
			errorLogger.Error("caught unknown signal, exiting", "signal", s)
			os.Exit(255)
		}
	}
}
