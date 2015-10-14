resource "docker_image" "appmapper" {
    name = "quay.io/weaveworks/app-mapper:latest"
    keep_updated = false
}

resource "docker_container" "appmapper" {
    count = "${var.appmapper_count}"
    image = "${docker_image.appmapper.latest}"
    name = "appmapper${count.index+1}"
    hostname = "app-mapper"
    domainname = "weave.local."
    command = [
      "-db-uri=${var.appmapper_database_uri}",
      "-docker-host=${var.appmapper_docker_host}",
      "-log-level=debug"
    ]
    # Needed in the local (laptop) environment
    volumes = {
      host_path = "/var/run/weave/weave.sock"
      container_path = "/var/run/weave/weave.sock"
    }
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
