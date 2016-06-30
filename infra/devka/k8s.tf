variable "k8s_node_count" {}
variable "k8s_node_instance_type" {}

module "kubernetes-anywhere-aws-ec2" {
    ## As the repository is currently seeing frequent changes, this module is reference by branch, and
    ## this will be kept up to date with relevant improvements, but shall not get any braking changes
    source                = "git::https://github.com/errordeveloper/kubernetes-anywhere//phase1/aws-ec2-terraform?ref=compat"
    phase2_implementation = "simple-weave-single-master"
    ec2_key_name          = "${aws_key_pair.kubernetes.key_name}"

    aws_access_key        = "${var.aws_access_key}"
    aws_secret_key        = "${var.aws_secret_key}"
    aws_region            = "${var.aws_region}"
    cluster               = "${var.env_name}"
    node_count            = "${var.k8s_node_count}"
    node_instance_type    = "${var.k8s_node_instance_type}"
}

resource "aws_key_pair" "kubernetes" {
    key_name   = "kubernetes-${var.env_name}"
    public_key = "${file("k8s_id_rsa.pub")}"
}
