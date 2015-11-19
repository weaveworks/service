# This file was copied from a template.
# You should not need to edit this file directly.
# Instead, use the accompanying tfvars file.

variable "route53_zone_name" {
    description = "Name of the Route53 zone."
}

variable "route53_frontend_elb_endpoint" {
    description = "ELB fronting to the Kubernetes service, via 'kubectl describe svc'."
}

variable "route53_frontend_elb_zone_id" {
    description = "CanonicalHostedZoneNameID for the ELB, via 'aws elb describe-load-balancers'."
}

variable "route53_record_name" {
    description = "Domain name for the Kubernetes frontend service, e.g. 'staging.weave.works'."
}

resource "aws_route53_zone" "zone" {
   name = "${var.route53_zone_name}"
   lifecycle {
     prevent_destroy = true
   }
}

resource "aws_route53_record" "www" {
    zone_id = "${aws_route53_zone.zone.zone_id}"
    name = "${var.route53_record_name}"
    type = "A"
    alias {
        name = "${var.route53_frontend_elb_endpoint}"
        zone_id = "${var.route53_frontend_elb_zone_id}"
        evaluate_target_health = false
    }
}
