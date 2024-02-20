## Redbus service scala API and SDK.

### Compile

Run to build (working in Java 8).

```shell
sbt compile
```

### Publish

Run to update maven package.

```shell
pushd ../../.. && make export-env && popd
sbt publish
```