package zuora

import (
	"context"

	"github.com/weaveworks/common/logging"
)

const (
	productsPath  = "catalog/products"
	weaveCloudSKU = "SKU-00000001"

	// DefaultCurrency represents Weave Cloud's default currency
	DefaultCurrency = "USD"
)

// RateMap maps product names to their rates.
type RateMap map[string]float64

type zuoraCatalogResponse struct {
	genericZuoraResponse
	// Many fields omitted below
	Products []struct {
		ID               string `json:"id"`
		Name             string `json:"name"` // "Weave Cloud",
		SKU              string `json:"sku"`  // "SKU-00000001"
		ProductRatePlans []struct {
			ID                     string `json:"id"`
			Status                 string `json:"status"` //: "Active",
			Name                   string `json:"name"`   //: "Weave Cloud SaaS | Node Usage",
			ProductRatePlanCharges []struct {
				ID      string `json:"id"` //: "2c92c0f95c86a44a015c8720ed265206",
				Name    string `json:"name"`
				UOM     string `json:"uom"` //: "node-seconds",
				Pricing []struct {
					Price    float64 `json:"price"`
					Currency string  `json:"currency"`
				} `json:"pricing"`
			} `json:"productRatePlanCharges"`
		} `json:"productRatePlans"`
	} `json:"products"`
}

// GetCurrentRates get the current product rates in USD from Zuora.
func (z *Zuora) GetCurrentRates(ctx context.Context) (RateMap, error) {
	resp := &zuoraCatalogResponse{}
	err := z.Get(ctx, productsPath, z.URL(productsPath), resp)
	if err != nil {
		return nil, err
	}
	rates := RateMap{}

	product := resp.Products[0]
	// Find Weave Cloud product in case there are other products
	for _, p := range resp.Products {
		if p.SKU == weaveCloudSKU {
			product = p
			break
		}
	}
	if product.SKU != weaveCloudSKU {
		logging.With(ctx).Warnf("Cannot find Zuora product %s", weaveCloudSKU)
	}

	// Collect all the charges across rate plans
	for _, ratePlan := range product.ProductRatePlans {
		for _, charge := range ratePlan.ProductRatePlanCharges {
			// Pricing is per currency which we are supposed to only have one of (USD).
			for _, pricing := range charge.Pricing {
				if pricing.Currency == DefaultCurrency {
					rates[charge.UOM] = pricing.Price
				}
			}
		}
	}

	return rates, err
}
