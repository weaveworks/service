resource "docker_image" "frontend" {
    name = "weaveworks/frontend:latest"
    keep_updated = false
}

resource "docker_container" "frontend" {
    count = 1
    image = "${docker_image.frontend.latest}"
    name = "frontend${count.index+1}"
    hostname = "frontend"
    domainname = "weave.local."
    ports = {
      internal = 80
      external = 80
    }
    must_run = true
}
