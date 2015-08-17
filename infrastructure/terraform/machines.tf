provider "aws" {
    access_key = "${var.access_key}"
    secret_key = "${var.secret_key}"
    region = "${var.region}"
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

    # outbound internet access
    egress {
        from_port = 0
        to_port = 0
        protocol = "-1"
        cidr_blocks = ["0.0.0.0/0"]
    }

    // These are for internal traffic
    ingress {
        from_port = 0
        to_port = 65535
        protocol = "tcp"
        self = true
    }

    ingress {
        from_port = 0
        to_port = 65535
        protocol = "udp"
        self = true
    }
}

resource "aws_instance" "docker-server" {
    ami = "ami-2b72a840"
    instance_type = "m3.large"
    count = "${var.servers}"

    connection {
      user = "${var.user}"
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
            "sudo sed -i -e's%-H fd://%-H fd:// -H tcp://0.0.0.0:2375 --fixed-cidr=172.17.42.1/24 -H unix:///var/run/docker.sock  -s overlay%' /lib/systemd/system/docker.service",
            "sudo systemctl daemon-reload",
            "sudo systemctl restart docker",
            "sudo systemctl enable docker",
        ]
    }
}

