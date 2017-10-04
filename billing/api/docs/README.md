# weave-cloud-billing

## Design overview

There are two billing-related services: API and Usage. The Usage service is responsible for maintaining and submitting usage data to the payment provider. The API service is responsible for account-related operations with the payment provider. Below is a overview of the two services.

![Billing overview image](images/Billing-overview.png)

## Usage

The configuration of the service is well documented by the --help cli argument. Consult this for up to date information.

### Configuration

Configuration can be performed via cli arguments, a config.yml file or environmental variables (periods replaced with underscores).

## API

See the [swagger api](../billing-service.yml) for details about the API. But generally the api is split into three methods:

- /accounts - Responsible for creating, getting and altering accounts
- /accounts{id}/invoices - Responsible for returning invoices
- /payments - Responsible for providing credit-card related functions (e.g. HPM form parameters and updating the credit card)

## Monitoring

All endpoints pass through prometheus monitoring middlewares.
