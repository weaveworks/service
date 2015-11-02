# Notes

## AWS networking

Kubernetes creates all of these except the default route table.
Arrows indicate foreign-key relationships.

```
digraph G {
    node [shape=rect];
    rankdir=LR;
    ranksep=1;

    {rank=same; vpc;}
    {rank=same; sg_master; sg_minion; acl; subnet; igw; rtb_k8s; rtb_default;}
    {rank=same; ec2_master; ec2_minions;}

	vpc -> sg_master;
	vpc -> sg_minion;
	vpc -> acl;
	vpc -> subnet;
	vpc -> igw;
	vpc -> rtb_k8s;
	vpc -> rtb_default;

	sg_master -> vpc;

	sg_minion -> vpc;

	acl -> vpc;
	acl -> subnet;

	subnet -> vpc;
	subnet -> acl;
	subnet -> rtb_k8s;

	igw -> vpc;

	rtb_k8s -> vpc;
	rtb_k8s -> subnet;
	rtb_k8s -> igw;
	rtb_k8s -> ec2_minions;

	rtb_default -> vpc;

	ec2_minions -> vpc;
	ec2_minions -> sg_minion;

	ec2_master -> vpc;
	ec2_master -> sg_master;
}
```
