package eventmanager

import fluxevent "github.com/weaveworks/flux/event"

// DeployData is data for release event
type DeployData fluxevent.ReleaseEventMetadata

// AutoDeployData is data for auto release event
type AutoDeployData fluxevent.AutoReleaseEventMetadata

// SyncData is data for sync event
type SyncData fluxevent.SyncEventMetadata

// CommitData is data for release commit, auto release commit and policy update commit events
type CommitData fluxevent.CommitEventMetadata

//TODO Move rendering from flux to notification
