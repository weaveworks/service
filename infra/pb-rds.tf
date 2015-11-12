# This file is automatically copied from the rds.tf.template file.
# Do not edit this file by hand.

resource "aws_db_instance" "users_db" {
    identifier = "users-db"
    allocated_storage = 10
    multi_az = true
    engine = "postgres"
    engine_version = "9.4.4"
    instance_class = "db.m1.small"
    username = "postgres"
    password = "${var.users_db_password}"
    vpc_security_group_ids = ["${var.minion_security_group_id}"]
    final_snapshot_identifier = "users-final"
}

resource "aws_db_instance" "app_mapper_db" {
    identifier = "app-mapper-db"
    allocated_storage = 10
    multi_az = true
    engine = "postgres"
    engine_version = "9.4.4"
    instance_class = "db.m1.small"
    username = "postgres"
    password = "${var.app_mapper_db_password}"
    vpc_security_group_ids = ["${var.minion_security_group_id}"]
    final_snapshot_identifier = "app-mapper-final"
}
