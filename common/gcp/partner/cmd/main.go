// This module is a CLI tool to interact with the Google Partner API.
// This is mostly useful when testing or being on-call, in order to manually approve subscriptions.
// Usage:
// - go to service-conf/k8s/<env>/default/gcp-launcher-secret.yaml
// - copy the value for "cloud-launcher.json"
// - run: $ echo -n "<value copied>" | base64 --decode > ~/.ssh/cloud-launcher-<env>.json
// - run: $ go run common/gcp/partner/cmd/main.go \
//             -partner-subscriptions-api.service-account-key-file=~/.ssh/cloud-launcher-<env>.json \
//             -action=approve \
//             -account-id=FOO \
//             -subscription-id=BAR
//
package main

import (
	"context"
	"flag"
	"fmt"

	log "github.com/sirupsen/logrus"
	"github.com/weaveworks/service/common/gcp/partner"
)

type config struct {
	action            string
	externalAccountID string
	subscriptionID    string
	partner           partner.Config
}

func (c *config) RegisterFlags(f *flag.FlagSet) {
	flag.StringVar(&c.action, "action", "approve", "Action to perform on the provided GCP account-subscription pair.")
	flag.StringVar(&c.externalAccountID, "external-account-id", "X-XXXX-XXXX-XXXX-XXXX", "GCP external account ID.")
	flag.StringVar(&c.subscriptionID, "subscription-id", "partnerSubscriptions/XXXXXXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX", "GCP subscription ID.")
	c.partner.RegisterFlags(f)
}

func main() {
	var cfg config
	cfg.RegisterFlags(flag.CommandLine)
	flag.Parse()

	partner, err := partner.NewClient(cfg.partner)
	if err != nil {
		log.Fatalf("Failed creating Google Partner Subscriptions API client: %v", err)
	}

	switch cfg.action {
	case "approve":
		approve(partner, cfg.externalAccountID, cfg.subscriptionID)
	default:
		fmt.Printf("Unknown command [%v].", cfg.action)
	}
}

func approve(client *partner.Client, externalAccountID, subscriptionID string) error {
	ctx := context.Background()
	body := partner.RequestBodyWithSSOLoginKey(externalAccountID)
	if _, err := client.ApproveSubscription(ctx, subscriptionID, body); err != nil {
		return err
	}
	return nil
}
