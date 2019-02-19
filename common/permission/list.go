package permission

// InviteTeamMember permission allows inviting new team members
const InviteTeamMember = "team.member.invite"

// DeleteInstance permission allows deleting team instances
const DeleteInstance = "instance.delete"

// UpdateBilling permission allows updating billing information
const UpdateBilling = "instance.billing.update"

// UpdateAlertingSettings permission allows editing alerting rules
const UpdateAlertingSettings = "alert.settings.update"

// UpdateTeamMemberRole permission allows updating team role of other team members
const UpdateTeamMemberRole = "team.member.update"

// RemoveTeamMember permission allows removing members from the team
const RemoveTeamMember = "team.member.remove"

// ViewTeamMembers permission allows viewing all the team members and their roles
const ViewTeamMembers = "team.members.view"

// TransferInstance permission allows transferring instances between the teams
const TransferInstance = "instance.transfer"

// CreateNotebook permission allows creating new notebooks
const CreateNotebook = "notebook.create"

// UpdateNotebook permission allows updating notebooks
const UpdateNotebook = "notebook.update"

// DeleteNotebook permission allows deleting notebooks
const DeleteNotebook = "notebook.delete"

// OpenHostShell permission allows opening shell on Scope host nodes
const OpenHostShell = "scope.host.exec"

// OpenContainerShell permission allows opening shell on Scope container nodes
const OpenContainerShell = "scope.container.exec"

// AttachToContainer permission allows attaching to Scope container nodes (just read-only mode is currently supported)
const AttachToContainer = "scope.container.attach.out"

// UpdateReplicaCount permission allows updating the number of K8s replicas
const UpdateReplicaCount = "scope.replicas.update"

// DeletePod permission allows deleting pods
const DeletePod = "scope.pod.delete"

// DeployImage permission allows deploying new images
const DeployImage = "flux.image.deploy"

// UpdateDeploymentPolicy permission allows updating policies of Flux workloads
const UpdateDeploymentPolicy = "flux.policy.update"

// UpdateNotificationSettings permission allows updating notification settings
const UpdateNotificationSettings = "notification.settings.update"

// ViewToken permission allows view the probe token
const ViewToken = "instance.token.view"

// ViewPodLogs permission allows viewing logs of K8s pods
const ViewPodLogs = "scope.pod.logs.view"

// PauseContainer permission allows pausing and unpausing Docker containers
const PauseContainer = "scope.container.pause"

// RestartContainer permission allows pausing Docker containers
const RestartContainer = "scope.container.restart"

// StopContainer permission allows pausing Docker containers
const StopContainer = "scope.container.stop"

// CreateWebhook permission allows user to create webhooks
const CreateWebhook = "instance.webhook.create"

// DeleteWebhook permission allows user to delete webhooks
const DeleteWebhook = "instance.webhook.delete"
