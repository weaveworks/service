package publisher

import "flag"

type Config struct {
	projectID             string
	topicID               string
	topicProjectID        string
	serviceAccountKeyFile string

	// CreateTopic says whether to attempt to create the topic. Needs permission to check for
	// existence of the topic in the topic project.
	CreateTopic bool
}

func (c *Config) RegisterFlags(f *flag.FlagSet) {
	name := "pubsub-api"
	flag.StringVar(&c.projectID, name+".project-id", "weaveworks-public", "Project for Pub/Sub access")
	flag.StringVar(&c.topicID, name+".topic-id", "weaveworks-public-cloudmarketplacepartner.googleapis.com", "Topic ID for the Pub/Sub subscription")
	flag.StringVar(&c.topicProjectID, name+".topic-project-id", "cloud-billing-subscriptions", "Only pass if topic is under another project")
	flag.StringVar(&c.serviceAccountKeyFile, name+".service-account-key-file", "", "Service account key JSON file")
}
