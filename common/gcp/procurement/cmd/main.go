// This module is a CLI tool to interact with the Google Partner API.
// This is mostly useful when testing or being on-call, in order to manually approve subscriptions.
// Usage:
// - go to service-conf/k8s/<env>/default/gcp-launcher-secret.yaml
// - copy the value for "cloud-launcher.json"
// - run: $ echo -n "<value copied>" | base64 --decode > ~/.ssh/cloud-launcher-<env>.json
// - run: $ go run common/gcp/partner/cmd/main.go \
//             -partner-procurement-api.service-account-key-file=~/.ssh/cloud-launcher-<env>.json \
//             -action=approve \
//             -account-id=FOO \
//             -subscription-id=BAR
//
package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/weaveworks/service/common/gcp/procurement"

	log "github.com/sirupsen/logrus"
)

type config struct {
	action             string
	entitlementName    string
	entitlementNewPlan string
	procurement        procurement.Config
}

func (c *config) RegisterFlags(f *flag.FlagSet) {
	flag.StringVar(&c.action, "action", "", "Action to perform on the provided GCP account-subscription pair.")
	flag.StringVar(&c.entitlementName, "entitlement-name", "providers/XXXXX/entitlements/XXXXXXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX", "GCP entitlement name.")
	flag.StringVar(&c.entitlementNewPlan, "entitlement-new-plan", "", "When doing approvePlanChange, the name of the new plan (standard, enterprise).")
	c.procurement.RegisterFlags(f)
}

func main() {
	var cfg config
	cfg.RegisterFlags(flag.CommandLine)
	flag.Parse()

	client, err := procurement.NewClient(cfg.procurement)
	if err != nil {
		log.Fatalf("Failed creating Google Partner Procurement API client: %v", err)
	}

	ctx := context.Background()
	switch cfg.action {
	case "get":
		ent, err := client.GetEntitlement(ctx, cfg.entitlementName)
		if err != nil {
			log.Fatal(err)
			return
		}
		fmt.Printf("Entitlement: %#v\n", ent)
	case "approve":
		if err := client.ApproveEntitlement(ctx, cfg.entitlementName); err != nil {
			log.Fatal(err)
			return
		}
	case "approvePlanChange":
		if err := client.ApprovePlanChangeEntitlement(ctx, cfg.entitlementName, cfg.entitlementNewPlan); err != nil {
			log.Fatal(err)
			return
		}
	default:
		fmt.Printf("Unknown command [%v].", cfg.action)
	}
}
