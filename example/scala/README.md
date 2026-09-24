# Scala REDBus example

The example builds against the Scala SDK from this checkout, so publishing the SDK first is not required.
Run the SDK and example compile gate from the repository root:

```shell
make scala-proto-compile
```

## Consumer

```shell
sbt "runMain example.Consumer topic-1 group-1"
```

## Producer

```shell
sbt "runMain example.Producer topic-1 \"Message text\""
```

```shell
sbt "runMain example.ProducerOutbox topic-1 \"Message text\""
```
