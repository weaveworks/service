provider "aws" {
  access_key = "AKIAJ7FURWZ4EFA4RUDA"
  secret_key = "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"
  region = "eu-central-1"
}

module "kubernetes-anywhere-aws-ec2" {
    source         = "github.com/weaveworks/weave-kubernetes-anywhere/examples/aws-ec2-terraform"
    aws_access_key = "AKIAJ7FURWZ4EFA4RUDA"
    aws_secret_key = "60DRnBPPbLn7drqhLVBf4Bkv4c5r8iPLyEpkSxJy"
    aws_region     = "eu-central-1"

    cluster        = "devc"
}

module "databases" {
    source         = "./databases"
    aws_access_key = "AKIAJ7FURWZ4EFA4RUDA"
    aws_secret_key = "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"
    aws_region     = "eu-central-1"

    cluster        = "devc"
    rds_sg_id      = "${module.kubernetes-anywhere-aws-ec2.kubernetes-main-sg-id}"
    users_db_password = "29fcf745896ac1f83c15ea03a22afcc2e4851048"
    app-mapper_db_password = "c798bb333cf5d9c211921e0682c1fcdb507d1e85"
}
