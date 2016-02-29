variable "postgres_version" {
    default = "9.4.4"
}

variable "cluster" {}
variable "rds_sg_id" {}

variable "users_db_password" {}
variable "app-mapper_db_password" {}

variable "aws_access_key" {}
variable "aws_secret_key" {}

variable "aws_region" {
    default = "us-east-1"
}

provider "aws" {
    access_key = "${var.aws_access_key}"
    secret_key = "${var.aws_secret_key}"
    region = "${var.aws_region}"
}

resource "aws_db_instance" "users_database" {
    identifier = "users-database-${var.cluster}"
    allocated_storage = 10
    multi_az = false
    engine = "postgres"
    engine_version = "${var.postgres_version}"
    instance_class = "db.t2.small"
    username = "postgres"
    password = "${var.users_db_password}"
    vpc_security_group_ids = ["${var.rds_sg_id}"]
    final_snapshot_identifier = "users-final"
    backup_retention_period = 5
    apply_immediately = true
}

resource "aws_db_instance" "app_mapper_database" {
    identifier = "app-mapper-database-${var.cluster}"
    allocated_storage = 10
    multi_az = false
    engine = "postgres"
    engine_version = "${var.postgres_version}"
    instance_class = "db.t2.small"
    username = "postgres"
    password = "${var.app-mapper_db_password}"
    vpc_security_group_ids = ["${var.rds_sg_id}"]
    final_snapshot_identifier = "app-mapper-final"
    backup_retention_period = 5
    apply_immediately = true
}
