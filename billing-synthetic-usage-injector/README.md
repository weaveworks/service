# `billing-synthetic-usage-injector`

## What? Why?

The `billing-synthetic-usage-injector` mocks a Weave Cloud instance and generates fake usage, a.k.a. "synthetic load".
This synthetic load is useful to test the billing pipeline end-to-end, at runtime.

## What is mocked exactly?

At this stage, only the Scope Probe running in users clusters is generating load which is then into usage data, and ultimately taken into account by our billing model, therefore, only the Scope Probe is mocked at the moment.
If we change our billing model to take other usage data in input, then the load injecter should be updated accordingly.

## Why not just gather usage from our `dev` or `prod` environments? Or some other real instance?

The two approaches complement each others.
A real instance is, well, more realistic.
However,

- the data generated may be less predictable,
- it may become expensive to test all scenari with real instances, and
- we may not be able to test some at all anyway.

Generating synthetic load lets us restrict the number of variables for our testing, specifically, it allows us to lock variables related to data, and prevents cases of "garbage in, garbage out":

- we know what `usage` gets in,
- we know the expected billing pipeline `f(g(h(i(j(.)))))`, s.t.:
  - `j`: Scope App, running server side
  - `i`: BigQuery,
  - `h`: `billing-aggregator,
  - `g`: `billing-uploader,
  - `f`: Billing backend (Zuora, GCP),
- we know how much `bill` we expect to get out.

So if `f(g(h(i(j(usage))))) != bill`, then we know something is wrong.
