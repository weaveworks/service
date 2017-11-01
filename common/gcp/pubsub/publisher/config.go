package publisher

import "flag"

// Config holds the configuration for a publisher client.
type Config struct {
	ProjectID             string
	TopicID               string
	TopicProjectID        string
	ServiceAccountKeyFile string

	// CreateTopic says whether to attempt to create the topic. Needs permission to check for
	// existence of the topic in the topic project.
	CreateTopic bool
}

// RegisterFlags register configuration.
func (c *Config) RegisterFlags(f *flag.FlagSet) {
	name := "pubsub-api"
	flag.StringVar(&c.ProjectID, name+".project-id", "weaveworks-public", "Project for Pub/Sub access")
	flag.StringVar(&c.TopicID, name+".topic-id", "weaveworks-public-cloudmarketplacepartner.googleapis.com", "Topic ID for the Pub/Sub subscription")
	flag.StringVar(&c.TopicProjectID, name+".topic-project-id", "cloud-billing-subscriptions", "Only pass if topic is under another project")
	flag.StringVar(&c.ServiceAccountKeyFile, name+".service-account-key-file", "", "Service account key JSON file")
}
