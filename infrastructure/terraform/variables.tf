variable "access_key" {
    description = "Your AWS Access Key"
}

variable "secret_key" {
    description = "Your AWS Secret Key"
}

variable "region" {
    default = "us-east-1"
    description = "The region of AWS, for AMI lookups."
}

variable "key_name" {
    description = "SSH key name in your AWS account for AWS instances."
    default = "weave-keypair"
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

variable "environment" {
    default = "dev"
}

variable "domain" {
  default = {
    "dev" = "dev.weave.works"
    "prod" = "cloud.weave.works"
  }
}


variable "users_db_password" {
}

variable "app_mapper_db_password" {
}

variable "registry_bucket_name" {
  default = {
    "dev" = "weaveworks_registry_dev"
    "prod" = "weave_works_registry"
  }
}
