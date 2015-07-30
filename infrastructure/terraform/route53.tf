resource "aws_route53_zone" "zone" {
   name = "cloud.weave.works"
}

resource "aws_route53_record" "docker-record" {
    zone_id = "${aws_route53_zone.zone.zone_id}"
    name = "docker.cloud.weave.works"
    type = "A"
    ttl = "1"
    records = ["${aws_instance.docker-server.*.public_ip}"]
}

resource "aws_route53_record" "internal-record" {
    zone_id = "${aws_route53_zone.zone.zone_id}"
    name = "internal.cloud.weave.works"
    type = "A"
    ttl = "1"
    records = ["${aws_instance.docker-server.*.private_ip}"]
}
