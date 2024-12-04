# Reliable Easy Data BUS

[![License](https://img.shields.io/badge/license-MIT-green)](https://github.com/prokraft/redbus/blob/master/LICENSE)

<img src="./doc/logo.jpeg" height="347"/>

RED Bus allows you to publish messages and process them with control over the result. You can set a retry
strategy for each consumer and the number of retries after which a message will be marked as failed.
The administrator can start reprocessing of unsuccessfully processed messages after fixing the problem through the
web interface.

## Issue

When you use messages in your system to maintain eventual consistency, it's important that every message is processed, 
and you must handle errors and implement a retry algorithm in every service in your system. You also need a tool to 
view failed messages and the ability to reprocess on demand.

## Resolve

Produce messages from anywhere and process them with repeated retries.  
RED Bus will do the rest for you.

![RED Bus Diagram](http://www.plantuml.com/plantuml/proxy?src=https://raw.githubusercontent.com/sergiusd/redbus/master/doc/resolve.puml)

## Build

```shell
    make build
```

## How to try

1. Set configuration 

   See `config.json` and `config/config.go`. Environment variable overwrited json data.
   For development environment you might use `config.local.json`


2. Start essential environment

   ```shell
       docker-compose -f example/docker-compose.yml up   
   ```

3. Run data bus service

   ```shell
       ./bin/databus
   ```

4. Try consumer and producer

   - [GoLang client](./example/golang/README.md)
   - [Scala client](./example/scala/README.md)