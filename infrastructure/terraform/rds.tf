resource "aws_db_instance" "users_db" {
    identifier = "users-db"
    allocated_storage = 10
    multi_az = true
    engine = "postgres"
    engine_version = "9.4"
    instance_class = "db.m1.small"
    username = "postgres"
    password = "${var.users_db_password}"
    vpc_security_group_ids = ["${aws_security_group.default.id}"]
    final_snapshot_identifier = "users-final"
}

output "users_db_uri" {
    value = "${aws_db_instance.users_db.engine}://${aws_db_instance.users_db.username}:PASSWORD@${aws_db_instance.users_db.address}:${aws_db_instance.users_db.port}/users?sslmode=disable"
}

resource "aws_db_instance" "app_mapper_db" {
    identifier = "app-mapper-db"
    allocated_storage = 10
    multi_az = true
    engine = "postgres"
    engine_version = "9.4"
    instance_class = "db.m1.small"
    username = "postgres"
    password = "${var.app_mapper_db_password}"
    vpc_security_group_ids = ["${aws_security_group.default.id}"]
    final_snapshot_identifier = "app-mapper-final"
}

output "app_mapper_db_uri" {
    value = "${aws_db_instance.app_mapper_db.engine}://${aws_db_instance.app_mapper_db.username}:PASSWORD@${aws_db_instance.app_mapper_db.address}:${aws_db_instance.app_mapper_db.port}/app_mapper?sslmode=disable"
}
