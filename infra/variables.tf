variable "region" {
    default = "us-east-1"
    description = "The region of AWS, for AMI lookups."
}

variable "users_db_password" {
}

variable "app_mapper_db_password" {
}

variable "minion_security_group_id" {
}

variable "db_subnet_id" {
}
