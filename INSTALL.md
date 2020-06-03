# Installing Ligase

Ligase is designed to be a Cloud Native application. We recommend deploy Ligase via docker.

This document shows how to start up Ligase on a single machine

## Requirements

* Go 1.13 or higher
* Postgres 9.5 or higher
* Apache Kafka 0.10.2+

## Setup denpendent services
recommended way:

```bash
docker-compose up -d
```

## Build 

### Build for docker

```bash
./build.sh
docker build -t ligase .
```

### Build for local host

```bash
./build.sh
```

add those to **/etc/hosts** if you want to run ligase in your local host.

```shell
127.0.0.1  pg-master
127.0.0.1  zookeeper
127.0.0.1  kafka
127.0.0.1  redis
127.0.0.1  nats
```

## Configuration

Replace ./config/config.yaml with your own configuration if you didn't use the recommended way to setup denpendent services. 


## Run

## Run in docker

```sh
docker run --network ligase_default --expose 8008 --detach --name ligase ligase
```

### Run in local host

1. In order to run ligase in your local host, follow the steps in https://github.com/edenhill/librdkafka to install librdkafka first.

2. Then start ligase by:

```sh
export SERVICE_NAME=monolith
./start.sh
```