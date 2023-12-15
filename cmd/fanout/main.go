package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/spf13/pflag"
)

var (
	service       string
	listenAddress string

	mu  sync.RWMutex
	sg  []*api.ServiceRegistration
	cnt int
)

func init() {
	pflag.StringVar(&service, "service", "", "service to connect to")
	pflag.StringVar(&listenAddress, "listen-address", "[::1]:8080", "listener address")
	pflag.Parse()
}

// https://github.com/m13253/popub/blob/a96c877dbf168309d73525a7987dd0d26e5c03cc/popub-relay/main.go#L194-L198
func copyTCPConn(dst, src *net.TCPConn) {
	var err error
	_, err = io.Copy(dst, src)
	if err != nil {
		log.Printf("WRITE error: %v\n", err)
	}
	_ = src.CloseRead()
	_ = dst.CloseWrite()
}

func updateServices(c *api.Client) error {
	services, _, err := c.Services().Get(service, &api.QueryOptions{
		AllowStale: true,
		AuthToken:  os.Getenv("NOMAD_TOKEN"), // FIXME: always fetch the latest token?
	})
	if err != nil {
		return err
	}

	// TODO: remove canary services by tag
	// TODO: print service changes
	mu.Lock()
	defer mu.Unlock()
	sg = services
	return nil
}

func handleTCPConn(listener *net.TCPListener) error {
	conn, err := listener.AcceptTCP()
	if err != nil {
		log.Printf("ACCEPT error: %v\n", err)
		return err
	}

	mu.RLock()
	defer mu.RUnlock()

	// get a possible endpoint (round-robin)
	cnt = (cnt + 1) % len(sg)
	dst, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("%s:%d", sg[cnt].Address, sg[cnt].Port))

	// connect
	proxyConn, err := net.DialTCP("tcp", nil, dst)
	if err != nil {
		log.Printf("DIAL error: %v\n", err)
		return err
	}
	log.Printf("CONN accept: %s --> %s - %s --> %s(%s)\n", conn.RemoteAddr(), conn.LocalAddr(), proxyConn.LocalAddr(), dst.String(), proxyConn.RemoteAddr())

	go copyTCPConn(proxyConn, conn)
	go copyTCPConn(conn, proxyConn)
	return nil
}

func main() {
	socket := filepath.Join(os.Getenv("NOMAD_SECRETS_DIR"), "api.sock")

	// connect to Nomad API with our workload identity
	c, err := api.NewClient(&api.Config{
		Address:   "unix://" + socket,
		Region:    os.Getenv("NOMAD_REGION"),
		SecretID:  os.Getenv("NOMAD_TOKEN"),
		Namespace: os.Getenv("NOMAD_NAMESPACE"),
	})
	if err != nil {
		log.Printf("API error: %v\n", err)
		os.Exit(255)
	}

	// update upstreams
	go func() {
		for {
			err = updateServices(c)
			if err != nil {
				log.Printf("REFERSH error: %v\n", err)
			}
			time.Sleep(1 * time.Second)
		}
	}()

	// start listener
	lServer, err := net.ResolveTCPAddr("tcp", listenAddress)
	if err != nil {
		log.Printf("RESOLVE error: %v\n", err)
		os.Exit(255)
	}
	listener, err := net.ListenTCP("tcp", lServer)
	if err != nil {
		log.Printf("LISTEN error: %v\n", err)
		os.Exit(255)
	}

	// TCP connection handling
	for {
		_ = handleTCPConn(listener)
	}
}
