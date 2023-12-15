package main

import (
	"fmt"
	"github.com/hashicorp/nomad/api"
	"github.com/spf13/pflag"
	"os"
	"path/filepath"
	"time"
)

var (
	source         string
	service        string
	affinity       []string
	protocol       string
	listenProtocol string
	listenAddress  string
)

func init() {
	pflag.StringVar(&source, "source", "", "data source type")
	pflag.StringVar(&service, "service", "", "service to connect to")
	pflag.StringSliceVar(&affinity, "affinity", []string{}, "affinity")
	pflag.StringVar(&protocol, "protocol", "tcp", "service protocol")
	pflag.StringVar(&listenProtocol, "listen-protocol", "tcp", "listener protocol")
	pflag.StringVar(&listenAddress, "listen-address", "[::1]:8080", "listener address")
	pflag.Parse()
}

func main() {
	if source != "nomad" {
		panic("not supported")
	}

	socket := filepath.Join(os.Getenv("NOMAD_SECRETS_DIR"), "api.sock")
	for {
		_, err := os.Stat(socket)
		if err != nil {
			fmt.Println("waiting")
			time.Sleep(100 * time.Millisecond)
		} else {
			break
		}
	}

	// connect to Nomad API with our workload identity
	c, err := api.NewClient(&api.Config{
		Address: "unix://" + socket,
		//Region:    os.Getenv("NOMAD_REGION"),
		//SecretID:  os.Getenv("NOMAD_TOKEN"),
		//Namespace: os.Getenv("NOMAD_NAMESPACE"),
	})
	if err != nil {
		panic(err)
	}

	fmt.Println(c.Address())

	for {
		fmt.Println("Upstreams:")
		services, _, err := c.Services().Get(service, &api.QueryOptions{
			AllowStale: true,
			AuthToken:  os.Getenv("NOMAD_TOKEN"),
		})
		if err != nil {
			panic(err)
		}

		for _, s := range services {
			fmt.Println(s.Address, s.Port)
		}

		time.Sleep(5 * time.Second)
	}
}
