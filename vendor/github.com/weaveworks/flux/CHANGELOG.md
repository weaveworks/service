## 1.0.1 (2017-09-19)

### Improvements

- Flux daemon can be configured to populate the git commit author with
  the name of the requesting user
- When multiple flux daemons share the same configuration repository,
  each fluxd only sends Slack notifications for commits that affect
  its branch/path
- When a resource is locked the invoking user is recorded, along with
  an optional message
- When a new config repo is synced for the first time, don't send
  notifications for the entire commit history

### Fixes

- The `fluxctl identity` command only worked via the Weave Cloud
  service, and not when connecting directly to the daemon

## 1.0.0 (2017-08-22)

This release introduces significant changes to the way flux works:

- The git repository is now the system of record for your cluster
  state. Flux continually works to synchronise your cluster with the
  config repository
- Release, automation and policy actions work by updating the config
  repository

See https://github.com/weaveworks/flux/releases/tag/1.0.0 for full
details.

## 0.3.0 (2017-05-03)

Update to support newer Kubernetes (1.6.1).

### Potentially breaking changes

- Support for Kubernetes' ReplicationControllers is deprecated; please
  update these to Deployments, which do the same job but much better
  (see
  https://kubernetes.io/docs/user-guide/replication-controller/#deployment-recommended)
- The service<->daemon protocol is versioned. The daemon will now
  crash-loop, printing a warning to the log, if it tries to connect to
  the service with a deprecated version of the protocol.

### Improvements

-   Updated the version of `kubectl` bundled in the Flux daemon image,
    to work with newer (>1.5) Kubernetes.
-   Added `fluxctl save` command for bootstrapping a repo from an existing cluster
-   You can now record a message and username with each release, which
    show up in notifications

## 0.2.0 (2017-03-16)

More informative and helpful UI.

### Features

-   Lots more documentation
-   More informative output from `fluxctl release`
-   Added option in `fluxctl set-config` to generate a deploy key

### Improvements

-   Slack notifications are tidier
-   Support for releasing to >1 service at a time
-   Better behaviour when flux deploys itself
-   More help given for commonly encountered errors
-   Filter out Kubernetes add-ons from consideration
-   More consistent Prometheus metric labeling

See also https://github.com/weaveworks/flux/issues?&q=closed%3A"2017-01-27 .. 2017-03-15"

## 0.1.0 (2017-01-27)

Initial semver release.

### Features

-   Validate image release requests.
-   Added version command

### Improvements

-   Added rate limiting to prevent registry 500's
-   Added new release process
-   Refactored registry code and improved coverage

See https://github.com/weaveworks/flux/milestone/7?closed=1 for full details.

