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

    # HTTP access from anywhere
    ingress {
        from_port = 80
        to_port = 80
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
            "sudo apt-key adv --keyserver hkp://p80.pool.sks-keyservers.net:80 --recv-keys 58118E89F3A912897C070ADBF76221572C52609D",
            "sudo bash -c 'echo deb https://apt.dockerproject.org/repo ubuntu-vivid main > /etc/apt/sources.list.d/docker.list'",
            "sudo apt-get update -qq",
            "sudo apt-get install -q -y --force-yes --no-install-recommends docker-engine",
            "sudo usermod -a -G docker ubuntu",
            "sudo sed -i -e's%-H fd://%-H fd:// -H tcp://0.0.0.0:2375 --fixed-cidr=172.17.42.1/24 -H unix:///var/run/docker.sock  -s overlay%' /lib/systemd/system/docker.service",
            "sudo systemctl daemon-reload",
            "sudo systemctl restart docker",
            "sudo systemctl enable docker",
        ]
    }
}

