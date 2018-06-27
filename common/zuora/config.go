package zuora

import (
	"errors"
	"flag"
	"time"
)

// Config provides the values necessary to create a new zuoraAPI
type Config struct {
	Username     string
	Password     string
	Endpoint     string
	RestEndpoint string
	Timeout      time.Duration

	// Accounts
	SuspendPolicy        string
	ResumePolicy         string
	SubscriptionTermType string
	ProductRatePlanID    string

	// Payments
	HostedPaymentPageURI string
	HostedPaymentPageID  string
	PaymentGateway       string
	SupportedCards       string

	// Usage
	DateFormat string
}

// RegisterFlags registers flags to configure a Zuora client.
func (c *Config) RegisterFlags(f *flag.FlagSet) {
	f.StringVar(&c.Username, "zuora.username", "", "Username for the Zuora API")
	f.StringVar(&c.Password, "zuora.password", "", "Password for the Zuora API")
	f.StringVar(&c.Endpoint, "zuora.endpoint", "https://apisandbox-api.zuora.com/rest/v1/", "Endpoint for Zuora's API")
	f.StringVar(&c.RestEndpoint, "zuora.rest-endpoint", "https://rest.apisandbox.zuora.com/v1/", "Endpoint for Zuora's REST API")
	f.DurationVar(&c.Timeout, "zuora.timeout", 10*time.Second, "Timeout for requests to the Zuora API")

	// Accounts
	f.StringVar(&c.SuspendPolicy, "zuora.policy.suspend", "Today", "Must be one of {Today, EndOfLastInvoicePeriod, SpecificDate, FixedPeriodsFromToday}")
	f.StringVar(&c.ResumePolicy, "zuora.policy.resume", "Today", "Must be one of {Today, EndOfLastInvoicePeriod, SpecificDate, FixedPeriodsFromToday}")
	f.StringVar(&c.SubscriptionTermType, "zuora.policy.term-type", "EVERGREEN", "Must be one of {TERMED, EVERGREEN}")
	f.StringVar(&c.ProductRatePlanID, "zuora.product-rate-plan-id", "", "REQUIRED: Zuora product rate plan ID (the id of the rate plan to subscribe to)")

	// Payments
	f.StringVar(&c.HostedPaymentPageURI, "zuora.hosted-payment-page-uri", "https://apisandbox.zuora.com/apps/PublicHostedPageLite.do", "URI of the hosted payments page for zuora")

	f.StringVar(&c.HostedPaymentPageID, "zuora.hosted-payment-page-id", "", "Hosted payments page id for Zuora")
	f.StringVar(&c.PaymentGateway, "zuora.payment-gateway", "dev", "Payment gateway to use for Zuora")
	f.StringVar(&c.SupportedCards, "zuora.supported-cards", "AmericanExpress,JCB,Visa,MasterCard,Discover", "Supported payment methods for Zuora")

	// Usage
	f.StringVar(&c.DateFormat, "zuora.date-format", "01/02/2006", "Date format represented by the date Mon Jan 2 15:04:05 -0700 MST 2006")
}

// Validate returns an error if anything is wrong with the configuration.
func (c *Config) Validate(requirePlanID bool) error {
	if requirePlanID && c.ProductRatePlanID == "" {
		return errors.New("-zuora.subscription-plan-id cannot be empty")
	}
	return nil
}
