# Playbook

![Architecture](architecture.png)

## General Advice

- Check the logs!
- If experiencing high latency, general advice is to scale the service up
- If experiencing high error rates, general advice is to look at error rates of dependant services (recursively) until you find the culprit.

Common problems:
- Bad credentials for AWS services
- Service failing to start as they can't reach their dependancies
- Bad configuration

## Per service notes

### Frontend

**What**: Exclusively proxies requests to the AuthFE and Users service.  Terminates SSL.

**If showing errors or high latency**: look at downstream servicesfor high error rates or latencies.

Otherwise could be bad configuration, which is compile into the container.  Check the logs.

NB Nginx is setup to resolve hostnames on every request, so a failure in KubeDNS or misconfiguration or resolver could cause a complete outage.  We inject the IP address of KubeDNS from the the yaml.

### Authfe

**What**:
- Checks authentication on every request,
- Proxies through to the relevant component.
- Logs events to a sidecar, which then sends to bigquery.
- Adds header to downstream requests to identify organisation.

**If showing errors or high latency**: look at downstream servicesfor high error rates or latencies.

Very little configuration; its all in the code.

### Users

**What**:
- Manages signup and login work flow.
- Accepts requests directly from frontend, and from authfe.

### Collection

**What**:
- Takes reports and writes them to S3
- Write time-based index of reports to DynamoDB
- Also pushes report key to NATS for shortcut reports

**If showing errors or high latency**: look at Dynamodb / S3 / NATS.

Note service won't start if it can't contact NATS.  Check the logs.

### Query

**What**:
- Reads report keys from DynamoDB
- Then reads reports from S3
- Has in process cache of decoded, decompressed reports
- Listens on NATS for shortcut reports

**If showing errors or high latency**: look at Dynamodb / S3 / NATS.

Note service won't start if it can't contact NATS.  Check the logs.

### Controls

**What**:
- Accepts control requests (HTTP POSTs) from the UI, writes control requests to SQS, waits for responses
- Accepts WS connections from probes, listens for control requests on SQS, forwards  responses back to SQS

### Pipes

**What**:
- Accepts pipe websockets from the UI & probe
- Maintains state of WS termincation in Consul
- Uses consul to coordinate bridging WS connections
- Blindly copies everything from UI<->bridge conneciton<->probe

### Users DB

**What**:
- Stores user details, credentials, org mappings
- Is an AWS RDS instance

**Useful info**:
- Can get a DB shell using `./infra/rds`
- Can see stats, management operations in AWS console

### DynamoDB

**What**:
- Stores time-based index of reports

**Useful info**:
- QPS is statically provisioned by terraform.
- Will throttle (appearing as high latency) if you exceed this.

**If showing errors or high latency**:
- Increase Read/Write QPS in AWS console

### S3

**What**:
- Stores reports

### NATS

**What**: Stateless, ephemeral pub-sub message bus used to publishing shortcut report ids from collection service to query service.

### Consul

**What**:
- Stores pipe state
