resource "aws_s3_bucket" "weaveworks_registry" {
    bucket = "${lookup(var.registry_bucket_name, var.environment)}"
    acl = "private"
}

resource "aws_iam_user" "registry" {
    name = "registry"
}

resource "aws_iam_access_key" "registry" {
    user = "${aws_iam_user.registry.name}"
}

resource "aws_iam_user_policy" "registry" {
    user = "${aws_iam_user.registry.name}"
    name = "registry"
    policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": "s3:*",
      "Resource": [
        "arn:aws:s3:::${lookup(var.registry_bucket_name, var.environment)}",
        "arn:aws:s3:::${lookup(var.registry_bucket_name, var.environment)}/*"
      ]
    }
  ]
}
EOF
}
