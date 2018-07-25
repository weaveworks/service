package zuora

import (
	"context"

	"github.com/weaveworks/common/logging"
	"github.com/weaveworks/common/user"
)

const (
	productsPath  = "catalog/products"
	productPath   = "catalog/product/%s"
	weaveCloudSKU = "SKU-00000001"
)

// RateMap maps product names to their rates.
type RateMap map[string]map[string]float64

type zuoraProduct struct {
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
}

type zuoraCatalogResponse struct {
	genericZuoraResponse
	// Many fields omitted below
	Products []zuoraProduct `json:"products"`
}

type zuoraCatalogProductResponse struct {
	genericZuoraResponse
	zuoraProduct
}

// GetProductsUnitSet get the units of measure billable for a set of products
func (z *Zuora) GetProductsUnitSet(ctx context.Context, productIDs []string) (map[string]bool, error) {
	units := map[string]bool{}
	for _, productID := range productIDs {
		resp := &zuoraCatalogProductResponse{}
		err := z.Get(ctx, productsPath, z.URL(productPath, productID), resp)
		if err != nil {
			return nil, err
		}
		for _, ratePlan := range resp.ProductRatePlans {
			for _, charge := range ratePlan.ProductRatePlanCharges {
				units[charge.UOM] = true
			}
		}
	}
	return units, nil
}

// GetCurrentRates get the current product rates, in all supported currencies, from Zuora.
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
		user.LogWith(ctx, logging.Global()).Warnf("Cannot find Zuora product %s", weaveCloudSKU)
	}

	// Collect all the charges across rate plans
	for _, ratePlan := range product.ProductRatePlans {
		for _, charge := range ratePlan.ProductRatePlanCharges {
			// Pricing is per currency
			for _, pricing := range charge.Pricing {
				if _, ok := rates[charge.UOM]; !ok {
					rates[charge.UOM] = make(map[string]float64)
				}
				rates[charge.UOM][pricing.Currency] = pricing.Price
			}
		}
	}

	return rates, err
}
