resource "aws_iam_user" "sqs_readwriter" {
    name = "${var.env_name}_sqs_readwriter"
}

resource "aws_iam_access_key" "sqs_readwriter_key" {
    user = "${aws_iam_user.sqs_readwriter.name}"
}

resource "aws_iam_user_policy" "sqs_readwriter_policy" {
    name = "sqs_readwriter_policy"
    user = "${aws_iam_user.sqs_readwriter.name}"
    policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Action": "sqs:*",
      "Effect": "Allow",
      "Resource": "arn:aws:sqs:${var.aws_region}:*:${var.env_name}_control_*"
    }
  ]
}
EOF
}
