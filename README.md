# Reliable Easy Data BUS

[![License](https://img.shields.io/badge/license-MIT-green)](https://github.com/sergiusd/redbus/blob/master/LICENSE)

<img src="./doc/logo.jpeg" height="347"/>

RED Bus allows you to publish messages and process them with control over the result of processing. You can set a retry
strategy for each consumer and the number of retries after which a message will be marked as failed by that consumer.
The administrator can start reprocessing of unsuccessfully processed messages after fixing the problem through the
web interface.

## Issue

When you use messages in your system to maintain eventual consistency, it's important that every message is processed, 
and you must handle errors and implement a retry algorithm in every service in your system. You also need a tool to 
view failed messages and the ability to reprocess on demand.

## Resolve

Produce messages from anyway and process it with repeat retries.  
RED Bus will do the rest for you.

![RED Bus Diagram](http://www.plantuml.com/plantuml/proxy?src=https://raw.githubusercontent.com/sergiusd/redbus/master/doc/resolve.puml)

## Build

```shell
    make build
```

## How to try

1. Start essential environment

   ```shell
       docker-compose -f example/docker-compose.yml up   
   ```

2. Run data bus service

   ```shell
       ./bin/databus
   ```

3. Try consumer and producer

   - [GoLang client](./example/golang/README.md)
   - [Scala client](./example/scala/README.md) (isn't working yet)