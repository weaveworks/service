# This file was copied from a template.
# You should not need to edit this file directly.
# Instead, use the accompanying tfvars file.

variable "aws_access_key" {
}

variable "aws_secret_key" {
}

variable "region" {
    description = "AWS region."
}

variable "rds_vpc_cidr_block" {
    description = "The CIDR block for the entire RDS VPC."
    default = "172.110.0.0/16"
}

variable "rds_subnet_a_cidr_block" {
    description = "The CIDR block for the first subnet in the RDS VPC."
    default = "172.110.1.0/24"
}

variable "rds_subnet_a_availability_zone" {
    description = "The AZ for the first subnet in the RDS VPC."
}

variable "rds_subnet_b_cidr_block" {
    description = "The CIDR block for the second subnet in the RDS VPC."
    default = "172.110.2.0/24"
}

variable "rds_subnet_b_availability_zone" {
    description = "The AZ for the second subnet in the RDS VPC."
    default = "us-east-1b"
}

variable "rds_subnet_c_cidr_block" {
    description = "The CIDR block for the third subnet in the RDS VPC."
    default = "172.110.3.0/24"
}

variable "rds_subnet_c_availability_zone" {
    description = "The AZ for the third subnet in the RDS VPC."
    default = "us-east-1c"
}

variable "users_db_password" {
}

variable "app_mapper_db_password" {
}

provider "aws" {
    access_key = "${var.aws_access_key}"
    secret_key = "${var.aws_secret_key}"
    region = "${var.region}"
}

resource "aws_vpc" "rds_vpc" {
    cidr_block = "${var.rds_vpc_cidr_block}"
    enable_dns_hostnames = true
}

resource "aws_security_group" "rds_sg" {
    name = "rds_sg"
    description = "Security group for RDS instances"
    ingress {
        from_port = 0
        to_port = 5432
        protocol = "tcp"
        cidr_blocks = ["0.0.0.0/0"]
    }
    egress {
        from_port = 0
        to_port = 0
        protocol = "-1"
        cidr_blocks = ["0.0.0.0/0"]
    }
}

resource "aws_internet_gateway" "rds_igw" {
    vpc_id = "${aws_vpc.rds_vpc.id}"
}

resource "aws_route_table" "rds_rtb" {
    vpc_id = "${aws_vpc.rds_vpc.id}"
    route {
        cidr_block = "0.0.0.0/0"
        gateway_id = "${aws_internet_gateway.rds_igw.id}"
    }
}

resource "aws_subnet" "rds_subnet_a" {
    vpc_id = "${aws_vpc.rds_vpc.id}"
    cidr_block = "${var.rds_subnet_a_cidr_block}"
    availability_zone = "${var.rds_subnet_a_availability_zone}"
}

resource "aws_route_table_association" "rds_rtbassoc_a" {
    subnet_id = "${aws_subnet.rds_subnet_a.id}"
    route_table_id = "${aws_route_table.rds_rtb.id}"
}

resource "aws_subnet" "rds_subnet_b" {
    vpc_id = "${aws_vpc.rds_vpc.id}"
    cidr_block = "${var.rds_subnet_b_cidr_block}"
    availability_zone = "${var.rds_subnet_b_availability_zone}"
}

resource "aws_route_table_association" "rds_rtbassoc_b" {
    subnet_id = "${aws_subnet.rds_subnet_b.id}"
    route_table_id = "${aws_route_table.rds_rtb.id}"
}

resource "aws_subnet" "rds_subnet_c" {
    vpc_id = "${aws_vpc.rds_vpc.id}"
    cidr_block = "${var.rds_subnet_c_cidr_block}"
    availability_zone = "${var.rds_subnet_c_availability_zone}"
}

resource "aws_route_table_association" "rds_rtbassoc_c" {
    subnet_id = "${aws_subnet.rds_subnet_c.id}"
    route_table_id = "${aws_route_table.rds_rtb.id}"
}

resource "aws_db_subnet_group" "rds_subnet_group" {
    name = "rds_subnet_group"
    description = "DB subnet group for the databases"
    subnet_ids = ["${aws_subnet.rds_subnet_a.id}", "${aws_subnet.rds_subnet_b.id}", "${aws_subnet.rds_subnet_c.id}"]
}

resource "aws_db_instance" "users_db" {
    identifier = "users-db"
    allocated_storage = 10
    multi_az = true
    engine = "postgres"
    engine_version = "9.4.4"
    instance_class = "db.m1.small"
    username = "postgres"
    password = "${var.users_db_password}"
    vpc_security_group_ids = ["${aws_security_group.rds_sg.id}"]
    #final_snapshot_identifier = "users-final"
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
    vpc_security_group_ids = ["${aws_security_group.rds_sg.id}"]
    #final_snapshot_identifier = "app-mapper-final"
}
