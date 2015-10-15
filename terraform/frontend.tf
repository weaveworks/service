resource "docker_image" "frontend" {
    name = "quay.io/weaveworks/frontend:latest"
    keep_updated = false
}

resource "docker_container" "frontend" {
    count = "${var.frontend_count}"
    image = "${docker_image.frontend.latest}"
    name = "frontend${count.index+1}"
    hostname = "frontend"
    domainname = "weave.local."
    ports = {
      internal = 80
      external = 80
    }
    must_run = true
    restart_policy = "always"
}
