# Fanout

Dead simple service mesh for [Hashicorp Nomad](https://www.nomadproject.io/).

## Feature

Fanout exists as a sidecar task, listens on a local port, and proxies incoming connections to a Nomad service. 

Supported features:
- Protocol: TCP

## Usage

For example, we have a group of Redis server as our service being discovered:
```hcl2
job "redis" {
  group "redis" {
    count = 3 // high-available setup is left out
    network {
      port "redis" { to = 6379 }
    }
    service {
      provider = "nomad"
      name     = "redis"
      port     = "redis"
      canary_tags = ["fanout.canary=1"]
    }
    task "redis" {
      driver = "docker"
      config {
        image = "redis:7.2"
        ports = ["redis"]
      }
    }
  }
}
```

This exposes 3 available Redis endpoints. But our backend server does not support connection to multiple Redis backends. So we use Fanout to distribute the connections automatically:

```hcl2
job "backend" {
  group "rails" {
    network {
      mode = "bridge" // required to put all the tasks into a single network namespace
      port "http" { to = 3000 }
    }
    service {
      provider = "nomad"
      name     = "rails"
      port     = "http"
    }
    task "fanout-redis" {
      lifecycle {
        hook    = "prestart"
        sidecar = true
      }
      identity { file = true }
      driver = "docker"
      config {
        image      = "jamesits/fanout:latest"
        args       = ["--service", "redis", "--listen-address", "127.0.0.1:6379"]
      }
      kill_signal = "SIGQUIT"
    }
    task "rails" {
      driver = "docker"
      config {
        image      = "example.com/my-rails-app"
        command    = "rails"
        args       = ["server", "-b", "::"]
        ports      = ["http"]
      }
      env {
        REDIS_URL  = "redis://127.0.0.1:6379/1"
      }
    }
  }
}
```

## Deployment Notes

[The CNI reference plugins](https://github.com/containernetworking/plugins) must be installed for the bridge network to work. On Debian 12:

```shell
apt install containernetworking-plugins
```

Then config the Nomad clients to discover the plugins:
```hcl2
client {
  cni_path          = "/usr/lib/cni"
}
```