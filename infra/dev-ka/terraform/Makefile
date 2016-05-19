default: reapply

modules:
	terraform get -update


reapply:
	terraform taint -module=kubernetes-anywhere-aws-ec2 aws_launch_configuration.kubernetes-node-group
	terraform taint -module=kubernetes-anywhere-aws-ec2 aws_autoscaling_group.kubernetes-node-group
	terraform apply

destroy:
	terraform plan -destroy -out kill.tfstate
	terraform apply kill.tfstate
	rm -f kill.tfstate

init:
	terraform get
	terraform apply
