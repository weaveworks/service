variable "host" {
    description = "The address of the docker daemon we're deploying to (or swarm master). Format is same as docker -H."
}

variable "users_session_secret" {
}

variable "users_email_uri" {
}

variable "users_database_uri" {
}

variable "appmapper_database_uri" {
}

variable "dev_containers_count" {
    description = "Used to enable dev-only containers (e.g. DBs) in dev mode"
}

variable "frontend_count" {
    description = "Number of replicas of the frontend to deploy"
    default = 1
}
