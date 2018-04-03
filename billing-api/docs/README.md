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

## Testing

### Zuora

Zuora has various rules to allow for billing or not, but these rules aren't explicit, and if a billing run fails, Zuora typically doesn't say why.
This means that, when testing against Zuora, you may try/fail several times before getting it right, and being able to produce an invoice, which can be frustrating.

Below is a guide which aims to address this.

### Rule of Thumb

In general, you'll need to make sure that events happen in the right chronological order, e.g.:

```
  |----|----|----|----|----|----> time
  T0   Ts   Tu1  Tb1  Tu2  Tb2 ...
```

s.t.:

- `T0`: Instance creation in Weave Cloud. It may not exist in Zuora just yet.
- `Ts`: Subscription's "_Contract Effective Date_" (explained below).
- `Tu`: Usage upload's `STARTDATE` and `ENDDATE` (explained below).
- `Tb`: Billing run's "_Invoice Date_" and "_Target Date for the Bill Run_" dates (explained below).

This may seem obvious, but Zuora's UI is rather hard to navigate, slow, and their terminology obscure, so this is easy to get wrong.

#### Example

1. Let's say we create an instance on 2018-03-29. Before we add a payment method, this instance doesn't exist in Zuora.
2. You can add a payment method by using one of the many test credit cards available out there (just google "test credit card"). One which is easy to remember is 4111 1111 1111 1111, with any expiration date and CVV. Upon submission of the credit card, a customer account and a subscription are created in Zuora.
3. If you did 1. and 2. on the same day, given we offer a 30 days trial, the subscription's default "_Contract Effective Date_" then is 2018-04-28.
This means any usage uploaded and bill run triggered before that date will basically get ignored by Zuora.
4. Fortunately, we can change this "_Contract Effective Date_":

    - go to ["_Customers_" > "_Customer Accounts_"](https://apisandbox.zuora.com/apps/CustomerAccount.do?menu=Z-Billing),
    - click on the account corresponding to your instance (or click "_View_") -- e.g. [that one](https://apisandbox.zuora.com/apps/CustomerAccount.do?method=view&id=2c92c0fa626c790b016272b228697170)
    - scroll down to "_Subscriptions & Amendments_",
    - click on the "_Subscription Number_" (e.g. `A-S00006912`),
    - click on "_set activation dates_",
    - set, for example, "_01/01/2018_",
    - click "_save_".

   We've now set `Ts` way back in the past, which now leaves plenty of time to simulate usage, and trigger bill runs.

5. To upload some usage,

    - go to ["_Billing_" > "_Usage_"](https://apisandbox.zuora.com/apps/Usages.do),
    - click on "_add usage records_",
    - click on "_Choose file_" and select a CSV file with the below format:

            ACCOUNT_ID,UOM,QTY,STARTDATE,ENDDATE,SUBSCRIPTION_ID,CHARGE_ID,DESCRIPTION
            W9525c87e3f78116b74ba7237a4b8dfd,node-seconds,999999,dd/mm/yyyy,dd/mm/yyyy,A-S00006912,C-00007755,manual usage upload


        - `ACCOUNT_ID` can be found on the account page for your instance, under "_Basic Information_" (e.g. `W9525c87e3f78116b74ba7237a4b8dfd`),
        - `SUBSCRIPTION_ID` is the _Subscription Number_ we saw previously (e.g. `A-S00006912`),
        - `CHARGE_ID` can be found on the [subscription's page](https://apisandbox.zuora.com/apps/Subscription.do?method=view&id=2c92c0fa626c790b016272b228f2717a), under "_Product & Charges_" > "_Weave Cloud SaaS | Nodes_" > "_Charge Number_" (e.g. `C-00007755`) ;

    - click on "_submit_",
    - you should now see that usage listed at the top of the page, as "_Pending_".

6. We are now ready to trigger a bill run to generate an invoice for the above usage,

    -  go to ["_Billing_" > "_Bill Runs_"](https://apisandbox.zuora.com/apps/NewBillingRun.do),
    - click on "_new bill run_",
    - select the "_Single Customer Account_" tab,
    - enter the above account ID (e.g. `W9525c87e3f78116b74ba7237a4b8dfd`), and click "_go_",
    - under "_enter date_", select the nearest "first day of the month" following the date you entered for the usage (e.g. if we uploaded our usage with `02/03/2018`, then we'd select `01/04/2018`) for both "_Invoice Date_" and "_Target Date for the Bill Run_",
    - click "_create bill run_",
    - click "_OK_",
    - you should now see that bill run listed at the top of the page as "_Pending_",
    - wait from a few seconds to a few minutes and it should become "_Completed_", and show "_Total Customers Processed_" and "_Number of Invoice_" both to 1.

7. We are now ready to publish this invoice, so that we can have it listed under Weave Cloud,

    - click on [the bill run](https://apisandbox.zuora.com/apps/NewBillingRun.do?method=view&id=2c92c0f9626c87e2016272eee82f2e02) (or click "_View_"),
    - click "_post_" in the top right corner,
    - click "_OK_".

8. Go back to the billing page for your instance in Weave Cloud, and you should now see an invoice.
