# Reliable Easy Data BUS

<img src="./doc/logo.jpeg" height="347"/>

## Build

    make build

## How to try

1. Start essential environment

       docker-compose -f example/docker-compose.yml up   

2. Run data bus service

       ./bin/databus

3. Try consumer and producer

   - [GoLang client](./example/golang/README.md)
   - [Scala client](./example/scala/README.md)