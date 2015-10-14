resource "docker_image" "appmapper" {
    name = "quay.io/weaveworks/app-mapper:latest"
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
      "-docker-host=${var.appmapper_docker_host}",
      "-log-level=debug"
    ]
    must_run = true
}

resource "docker_image" "appmapper_db" {
    count = "${var.dev_containers_count}"
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
