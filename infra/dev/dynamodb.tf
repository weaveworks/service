resource "aws_dynamodb_table" "report_table" {
    name = "${var.env_name}_reports"
    read_capacity = 100
    write_capacity = 200
    hash_key = "hour"
    range_key = "ts"
    attribute {
      name = "hour"
      type = "S"
    }
    attribute {
      name = "ts"
      type = "N"
    }
}

resource "aws_s3_bucket" "report_bucket" {
    bucket = "weaveworks_${var.env_name}_reports"
}

resource "aws_iam_user" "report_writer" {
    name = "${var.env_name}_report_writer"
}

resource "aws_iam_access_key" "report_writer_key" {
    user = "${aws_iam_user.report_writer.name}"
}

resource "aws_iam_user_policy" "report_writer_policy" {
    name = "report_writer_policy"
    user = "${aws_iam_user.report_writer.name}"
    policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Action": [
         "dynamodb:PutItem"
      ],
      "Effect": "Allow",
      "Resource": "${aws_dynamodb_table.report_table.arn}"
    },
    {
      "Action": [
         "s3:PutObject"
      ],
      "Effect": "Allow",
      "Resource": "${aws_s3_bucket.report_bucket.arn}/*"
    }
  ]
}
EOF
}

resource "aws_iam_user" "report_reader" {
    name = "${var.env_name}_report_reader"
}

resource "aws_iam_access_key" "report_reader_key" {
    user = "${aws_iam_user.report_reader.name}"
}

resource "aws_iam_user_policy" "report_reader_policy" {
    name = "report_reader_policy"
    user = "${aws_iam_user.report_reader.name}"
    policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Action": [
         "dynamodb:Query",
         "dynamodb:GetItem",
         "dynamodb:BatchGetItem"
      ],
      "Effect": "Allow",
      "Resource": "${aws_dynamodb_table.report_table.arn}"
    },
    {
      "Action": [
         "s3:GetObject"
      ],
      "Effect": "Allow",
      "Resource": "${aws_s3_bucket.report_bucket.arn}/*"
    }
  ]
}
EOF
}
