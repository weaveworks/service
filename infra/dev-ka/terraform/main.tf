variable "aws_access_key" {
    default = "AKIAJ7FURWZ4EFA4RUDA"
}

variable "aws_secret_key" {
    default = "60DRnBPPbLn7drqhLVBf4Bkv4c5r8iPLyEpkSxJy"
}

variable "aws_region" {
    default = "us-east-1"
}

module "az" {
    source = "github.com/errordeveloper/tf_aws_availability_zones"
    region = "${var.aws_region}"
}

module "kubernetes-anywhere-aws-ec2" {
    source         = "github.com/weaveworks/weave-kubernetes-anywhere/examples/aws-ec2-terraform"
    aws_access_key = "${var.aws_access_key}"
    aws_secret_key = "${var.aws_secret_key}"
    aws_region     = "${var.aws_region}"

    cluster                = "dev01"
    cluster_config_flavour = "secure-v1.2"
}

module "databases" {
    source         = "../../terraform/databases"
    aws_access_key = "${var.aws_access_key}"
    aws_secret_key = "${var.aws_secret_key}"
    aws_region     = "${var.aws_region}"

    az1            = "${module.az.primary}"
    az2            = "${module.az.secondary}"

    cluster        = "devz" // TODO: same here
    rds_sg_id      = "${module.kubernetes-anywhere-aws-ec2.kubernetes-main-sg-id}"
    rds_vpc_id     = "${module.kubernetes-anywhere-aws-ec2.kubernetes-vpc-id}"

    users_db_password      = "29fcf745896ac1f83c15ea03a22afcc2e4851048"
    app_mapper_db_password = "c798bb333cf5d9c211921e0682c1fcdb507d1e85"
}
