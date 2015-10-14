resource "docker_image" "users" {
    name = "quay.io/weaveworks/users:latest"
    keep_updated = false
}

resource "docker_container" "users" {
    count = 1
    image = "${docker_image.users.latest}"
    name = "users${count.index+1}"
    hostname = "users"
    domainname = "weave.local."
    command = [
      "-session-secret", "${var.users_session_secret}",
      "-email-uri", "${var.users_email_uri}",
      "-database-uri", "${var.users_database_uri}"
    ]
    must_run = true
}

resource "docker_image" "users_db" {
    count = "${var.dev_containers_count}"
    name = "weaveworks/users-db:latest"
    keep_updated = false
}

resource "docker_container" "users_db" {
    count = "${var.dev_containers_count}"
    image = "${docker_image.users_db.latest}"
    name = "users-db"
    hostname = "users-db"
    domainname = "weave.local."
    must_run = true
}

resource "docker_image" "mailcatcher" {
    name = "schickling/mailcatcher:latest"
}

resource "docker_container" "mailcatcher" {
    count = "${var.dev_containers_count}"
    image = "${docker_image.mailcatcher.latest}"
    name = "mailcatcher"
    hostname = "smtp"
    domainname = "weave.local."
    command = [
      "mailcatcher", "-f", "--ip=0.0.0.0", "--smtp-port=587", "--http-port=80"
    ]
    must_run = true
}
