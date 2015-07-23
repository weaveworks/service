variable "key_name" {
    description = "Name of the SSH keypair to use in AWS."
    default = "weave-keypair"
}

variable "key_path" {
    description = "Path to the private portion of the SSH key specified."
    default = "weave-keypair.pem"
}

variable "hosts" {
    description = "Number of hosts to spin up"
    default = 1
}

provider "aws" {
    access_key = "AKIAJDHBWPNHE32K5KTA"
    secret_key = "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"
    region = "us-east-1"
}

resource "aws_security_group" "default" {
    name = "nodes"

    # SSH access from anywhere
    ingress {
        from_port = 22
        to_port = 22
        protocol = "tcp"
        cidr_blocks = ["0.0.0.0/0"]
    }

    # Weave proxy from anywhere
    ingress {
        from_port = 12375
        to_port = 12375
        protocol = "tcp"
        cidr_blocks = ["0.0.0.0/0"]
    }

    # outbound internet access
    egress {
        from_port = 0
        to_port = 0
        protocol = "-1"
        cidr_blocks = ["0.0.0.0/0"]
    }
}

module "consul" {
    source = "github.com/hashicorp/consul/terraform/aws"

    key_name = "${var.key_name}"
    key_path = "${var.key_path}"
    servers = "1"
}

resource "aws_instance" "node" {
    ami = "ami-5d25e336"
    instance_type = "m3.large"
    count = 1

    connection {
      user = "ubuntu"
      key_file = "${var.key_path}"
    }
    key_name = "${var.key_name}"

    security_groups = ["${aws_security_group.default.name}"]

    provisioner "remote-exec" {
        inline = [
            "sudo apt-key adv --keyserver hkp://keyserver.ubuntu.com:80 --recv-keys 36A1D7869245C8950F966E92D8576A8BA88D21E9",
            "sudo bash -c 'echo deb https://get.docker.io/ubuntu docker main > /etc/apt/sources.list.d/docker.list'",
            "sudo apt-get update -qq",
            "sudo apt-get install -q -y --force-yes --no-install-recommends lxc-docker",
            "sudo usermod -a -G docker ubuntu",
            "sudo bash -c 'echo DOCKER_OPTS=\\\"--fixed-cidr=172.17.42.1/24 -H unix:///var/run/docker.sock -H tcp://0.0.0.0:2375\\\" >> /etc/default/docker'",
            "sudo service docker restart",

            "sudo curl -L git.io/weave -o /usr/local/bin/weave",
            "sudo chmod a+x /usr/local/bin/weave",
            "sudo weave launch",
            "sudo weave launch-dns",
            "sudo weave launch-proxy",

            "sudo docker run -d --name=swarm-agent swarm join --advertise=172.17.42.1:12375 consul://${module.consul.server_address}/swarm",
        ]
    }
}

provider "docker" {
    host = "tcp://${aws_instance.node.0.public_dns}:12375"
}

# Create a container
resource "docker_container" "swarm-master" {
    depends_on = ["docker_image.swarm"]
    image = "swarm"
    name = "swarm-master"
    command = ["manage", "-H", "tcp://0.0.0.0:4567", "consul://${module.consul.server_address}/swarm"]
}

resource "docker_image" "swarm" {
  name = "swarm"
}
