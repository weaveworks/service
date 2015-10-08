resource "docker_image" "appmapper" {
    name = "weaveworks/app-mapper:latest"
    keep_updated = false
}

resource "docker_container" "appmapper" {
    count = 1
    image = "${docker_image.appmapper.latest}"
    name = "appmapper${count.index+1}"
    hostname = "app-mapper"
    domainname = "weave.local."
    command = [
      "-db-uri=${var.appmapper_database_uri}",
      "-docker-host=unix:///var/run/weave/weave.sock",
      "-log-level=debug"
    ]
    volumes = {
      host_path = "/var/run/weave/weave.sock"
      container_path = "/var/run/weave/weave.sock"
    }
    must_run = true
}

resource "docker_image" "appmapper_db" {
    name = "weaveworks/app-mapper-db:latest"
    keep_updated = false
}

resource "docker_container" "appmapper_db" {
    count = "${var.dev_containers_count}"
    image = "${docker_image.appmapper_db.latest}"
    name = "appmapper-db"
    hostname = "app-mapper-db"
    domainname = "weave.local."
    must_run = true
}
