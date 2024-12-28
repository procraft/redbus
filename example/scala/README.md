# Scala REDBus example

You should have published [API and SDK packages](../../api/scala/redbus/README.md).

```shell
cd ../.. && make export-env
```

## Consumer

```shell
sbt "runMain example.Consumer topic-1 group-1"
```

## Producer

```shell
sbt "runMain example.Producer topic-1 \"Message text\""
```