job "fanout" {
  type = "service"

//   group "envvar" {
//     task "envvar" {
//       driver = "raw_exec"
//       config {
//         command = "/bin/bash"
//         args = [
//           "-c",
//           "printenv",
//         ]
//       }
//     }
//   }

  group "server" {
    network {
      port "http" {}
    }

    service {
      provider = "nomad"
      name     = "http"
      port     = "http"
    }

    task "http-server" {
      driver = "raw_exec"
      config {
        command = "/bin/bash"
        args = [
          "-c",
          "sleep 10000000",
        ]
      }
    }
  }

  group "fanout" {
    task "fanout" {
      identity {
        env = true
      }

      driver = "raw_exec"
      config {
        command = "fanout"
        args = [
          "--source",
          "nomad",
          "--service",
          "http"
        ]
      }
    }
  }
}
