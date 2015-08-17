variable "access_key" {
}

variable "secret_key" {
}

variable "region" {
    default = "us-east-1"
    description = "The region of AWS, for AMI lookups."
}

variable "key_name" {
    description = "SSH key name in your AWS account for AWS instances."
}

variable "key_path" {
    description = "Path to the private key specified by key_name."
}

variable "servers" {
    default = "3"
    description = "The number of servers to launch."
}

variable "user" {
    default = "ubuntu"
}


