#!/bin/bash

terraform taint -module=kubernetes-anywhere-aws-ec2 aws_launch_configuration.kubernetes-node-group
terraform taint -module=kubernetes-anywhere-aws-ec2 aws_autoscaling_group.kubernetes-node-group
terraform apply
