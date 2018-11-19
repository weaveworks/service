package webhooks

// WebhooksIntegrationTypeHeader is the header used to pass the integration type from authfe to flux-api.
const WebhooksIntegrationTypeHeader = "X-Webhooks-Integration-Type"

// GithubPushIntegrationType for github pushes which will ask flux to sync
const GithubPushIntegrationType = "github.push"

// BitbucketOrgPushIntegrationType is for webhook endpoints that accept repo push notifications from BitBucket Cloud
const BitbucketOrgPushIntegrationType = "bitbucket.push"

// GitlabPushIntegrationType is for webhook endpoints that accept repo push notifications from GitLab
const GitlabPushIntegrationType = "gitlab.push"

// DockerHubIntegrationType for dockerhub pushes which will ask flux to sync
const DockerHubIntegrationType = "dockerhub.push"

// QuayIntegrationType for quay pushes which will ask flux to sync
const QuayIntegrationType = "quay.push"
