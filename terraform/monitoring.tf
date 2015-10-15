resource "docker_image" "monitoring" {
    name = "quay.io/weaveworks/monitoring:latest"
    keep_updated = false
}

resource "docker_container" "monitoring" {
    count = 1
    image = "${docker_image.monitoring.latest}"
    name = "monitoring${count.index+1}"
    hostname = "monitoring"
    domainname = "weave.local."
    must_run = true
    ports = {
      internal = 3000
      external = 3000
    }
    ports = {
      internal = 9090
      external = 9090
    }
    restart_policy = "always"
}
