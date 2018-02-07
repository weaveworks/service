# service-ui-kicker

`service-ui-kicker` automatically updates the `service-ui` repository in response to certain events.

## Scope version updates

`service-ui-kicker` updates the version of `scope` in the `service-ui` repository.

It considers only the last successfully built `weaveworks/scope` master commit, as indicated by github `Commit Status` webhook.

`yarn add` automatically updates `package.json` and `yarn.lock` files to the specified version of `weave-scope`.

After that `service-ui-kicker` creates and pushes new commit to `service-ui` repository with updated `client/package.json` and `client/yarn.lock` files.

In `client/package.json` file it changes value of the `dependencies.weave-scope`, for example `142d8bea` here:

```json
"weave-scope": "https://s3.amazonaws.com/weaveworks-js-modules/weave-scope/142d8bea/weave-scope.tgz"
```

## Build preview URLs

`service-ui-kicker` adds preview URLs to commits in the `service-ui` repository.

Build preview urls allow you to test out builds from `service-ui` against the `frontend.dev.weave.works` backend.

Preview URLs look like `https://1234.build.dev.weave.works`, where `1234` is the build ID of the CircleCI job which uploaded artefacts from a commit's build pipeline.

## Webhook settings

* Payload URL: `https://frontend.dev.weave.works/service-ui-kicker`
* Content type: `application/json`
* Events: `push`, `status`
