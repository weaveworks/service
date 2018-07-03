package webhooks

// WebhooksIntegrationTypeHeader is the header used to pass the integration type from authfe to flux-api.
const WebhooksIntegrationTypeHeader = "X-Webhooks-Integration-Type"

// GithubPushIntegrationType for github pushes which will ask flux to sync
const GithubPushIntegrationType = "github.push"
