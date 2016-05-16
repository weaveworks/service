package main

import (
	log "github.com/Sirupsen/logrus"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
)

func getEC2Nodes(region string, cluster string) ([]string, error) {
	svc := ec2.New(session.New(), &aws.Config{Region: aws.String(region)})

	params := &ec2.DescribeInstancesInput{
		Filters: []*ec2.Filter{
			{
				Name: aws.String("tag:KubernetesCluster"),
				Values: []*string{
					aws.String(cluster),
				},
			},
			{
				Name: aws.String("tag:Name"),
				Values: []*string{
					aws.String("kubernetes-node"),
				},
			},
			{
				Name: aws.String("instance-state-name"),
				Values: []*string{
					aws.String("running"),
				},
			},
		},
	}

	resp, err := svc.DescribeInstances(params)
	if err != nil {
		return nil, err
	}
	log.Println("Number of reservation sets:", len(resp.Reservations))

	var nodes []string
	for idx, res := range resp.Reservations {
		log.Println("Number of instances:", len(res.Instances))
		for _, inst := range resp.Reservations[idx].Instances {
			log.Println("Instance ID:", *inst.InstanceId)
			nodes = append(nodes, *inst.PublicIpAddress)
		}
	}
	return nodes, nil
}
