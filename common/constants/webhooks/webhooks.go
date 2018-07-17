package webhooks

// WebhooksIntegrationTypeHeader is the header used to pass the integration type from authfe to flux-api.
const WebhooksIntegrationTypeHeader = "X-Webhooks-Integration-Type"

// GithubPushIntegrationType for github pushes which will ask flux to sync
const GithubPushIntegrationType = "github.push"

// GithubPushIntegrationType for dockerhub pushes which will ask flux to sync
const DockerHubIntegrationType = "dockerhub.push"

// QuayIntegrationType for quay pushes which will ask flux to sync
const QuayIntegrationType = "quay.push"
