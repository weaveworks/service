resource "aws_route53_zone" "zone" {
   name = "${lookup(var.domain, var.environment)}"
   lifecycle {
     prevent_destroy = true
   }
}

resource "aws_route53_record" "docker-record" {
    zone_id = "${aws_route53_zone.zone.zone_id}"
    name = "docker.${lookup(var.domain, var.environment)}"
    type = "A"
    ttl = "1"
    records = ["${aws_instance.docker-server.*.public_ip}"]
}

resource "aws_route53_record" "internal-record" {
    zone_id = "${aws_route53_zone.zone.zone_id}"
    name = "internal.${lookup(var.domain, var.environment)}"
    type = "A"
    ttl = "1"
    records = ["${aws_instance.docker-server.*.private_ip}"]
}
