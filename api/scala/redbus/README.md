## Redbus service scala API and SDK.

### Compile

Run to build.

```shell
sbt compile
```

### Publish

Run to update maven package.

```shell
pushd ../../.. && make export-env && popd
sbt publish
```