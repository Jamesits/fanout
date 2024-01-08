package main

import (
	"github.com/hashicorp/nomad/api"
	"os"
	"path/filepath"
	"slices"
	"sync/atomic"
	"time"
)

const (
	identityRefreshInterval = 60 * time.Second
	serviceRefreshInterval  = 1 * time.Second
)

var nomadToken atomic.Pointer[string]

func updateWorkloadIdentity() {
	// https://developer.hashicorp.com/nomad/docs/concepts/workload-identity

	// if we are unable to read any token and this routine exits unexpectedly, fail the program
	defer func() {
		token := nomadToken.Load()
		if token == nil || *token == "" {
			errorLogger.Error("unable to read Nomad workload identity from any sources")
			os.Exit(255)
		}
	}()

	// first we try to read from environment variable
	token := os.Getenv("NOMAD_TOKEN")
	nomadToken.Store(&token)

	// if the token file does not exist, we are not able to continuously refresh the token
	p := filepath.Join(os.Getenv("NOMAD_SECRETS_DIR"), "nomad_token")
	if _, err := os.Stat(p); err != nil {
		errorLogger.Warn("Nomad workload identity file not found, fallback to environment variable")
		return
	}

	// refresh
	for {
		t, err := os.ReadFile(p)
		if err != nil || len(t) == 0 {
			errorLogger.Warn("failed refreshing workload identity from file", "error", err)
		} else {
			token = string(t)
			nomadToken.Store(&token)
		}
		time.Sleep(identityRefreshInterval)
	}
}

func updateServices() {
	// connect to Nomad API with our workload identity
	c, err := api.NewClient(&api.Config{
		Address:   "unix://" + filepath.Join(os.Getenv("NOMAD_SECRETS_DIR"), "api.sock"),
		Region:    os.Getenv("NOMAD_REGION"),
		Namespace: os.Getenv("NOMAD_NAMESPACE"),
	})
	if err != nil {
		errorLogger.Error("unable to create new Nomad API client", "error", err)
		os.Exit(255)
	}

	for {
		token := nomadToken.Load()
        if token == nil { // wait for workload identity to be read; TODO: refactor this with a proper sync primitive
            errorLogger.Debug("waiting for token retrieval")
            time.Sleep(10 * time.Millisecond)
            continue
        }
		services, _, err := c.Services().Get(service, &api.QueryOptions{
			AllowStale: allowStale,
			AuthToken:  *token,
		})
		if err == nil {
			var newSg []*api.ServiceRegistration
			for _, s := range services {
				if slices.Contains(s.Tags, "fanout.canary=1") {
					continue
				}

				newSg = append(newSg, s)
			}
			// TODO: print service changes
			mu.Lock()
			sg = newSg
			mu.Unlock()
		} else {
			errorLogger.Error("unable to load service endpoints from Nomad", "error", err)
		}

		time.Sleep(serviceRefreshInterval)
	}
}
