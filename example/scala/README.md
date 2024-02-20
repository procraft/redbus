# Scala REDBus example

You should have published [API and SDK packages](../../api/scala/redbus/README.md).

```shell
cd ../.. && make export-env
```

## Consumer

```shell
sbt "runMain example.Consumer"
```

## Producer

```shell
sbt "runMain example.Producer"
```