resource "docker_image" "ui-server" {
    name = "quay.io/weaveworks/ui-server:latest"
    keep_updated = false
}

resource "docker_container" "ui-server" {
    count = "${var.uiserver_count}"
    image = "${docker_image.ui-server.latest}"
    name = "ui-server${count.index+1}"
    hostname = "ui-server"
    domainname = "weave.local."
    must_run = true
    restart_policy = "always"
}
