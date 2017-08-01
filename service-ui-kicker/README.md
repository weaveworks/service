# service-ui-kicker

`service-ui-kicker` automatically updates version of `scope` in the `service-ui` repository.

It considers only the last successfully built `weaveworks/scope` master commit, as indicated by github `Commit Status` webhook.

`yarn add` automatically updates `package.json` and `yarn.lock` files to the specified version of `weave-scope`.

After that `service-ui-kicker` creates and pushes new commit to `service-ui` repository with updated `client/package.json` and `client/yarn.lock` files.

In `client/package.json` file it changes value of the `dependencies.weave-scope`, for example `142d8bea` here:

```json
"weave-scope": "https://s3.amazonaws.com/weaveworks-js-modules/weave-scope/142d8bea/weave-scope.tgz"
```

## Webhook settings

* Payload URL: `https://frontend.dev.weave.works/service-ui-kicker`
* Content type: `application/json`
* Events: `push`, `status`
