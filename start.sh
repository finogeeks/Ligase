#!/bin/sh
export LOG_DIR=/opt/ligase/log
export ENABLE_MONITOR=true
export MONITOR_PORT=7000

if [ "$SERVICE_NAME" = "front" ]
then
    cat /opt/ligase/config/config.yaml
    ./bin/engine-server --name=front-server --config=./config/config.yaml --http-address=$HTTP_ADDR --log-porf=true
elif [ "$SERVICE_NAME" = "persist" ]
then
    cat /opt/ligase/config/config.yaml
    ./bin/engine-server --name=persist-server --config=./config/config.yaml --http-address=$HTTP_ADDR --log-porf=true
elif [ "$SERVICE_NAME" = "as" ]
then
    cat /opt/ligase/config/as-registration.yaml
    ./bin/engine-server --name=app-service --config=./config/config.yaml --log-porf=true
elif [ "$SERVICE_NAME" = "push" ]
then
    cat /opt/ligase/config/config.yaml
    ./bin/engine-server --name=push-sender --config=./config/config.yaml --log-porf=true
elif [ "$SERVICE_NAME" = "loader" ]
then
    ./bin/engine-server --name=cache-loader --config=./config/config.yaml --http-address=$HTTP_ADDR --log-porf=true
elif [ "$SERVICE_NAME" = "monolith" ]
then
    cat /opt/ligase/config/config.yaml
    ./bin/engine-server --name=monolith-server --config=./config/config.yaml --http-address=$HTTP_ADDR --log-porf=true
elif [ "$SERVICE_NAME" = "sync-writer" ]
then
    cat /opt/ligase/config/config.yaml
    ./bin/engine-server --name=sync-writer --config=./config/config.yaml --log-porf=true
elif [ "$SERVICE_NAME" = "sync-aggregate" ]
then
    cat /opt/ligase/config/config.yaml
    ./bin/engine-server --name=sync-aggregate --config=./config/config.yaml --http-address=$HTTP_ADDR --log-porf=true
elif [ "$SERVICE_NAME" = "sync-server" ]
then
    cat /opt/ligase/config/config.yaml
    ./bin/engine-server --name=sync-server --config=./config/config.yaml --http-address=$HTTP_ADDR --log-porf=true
elif [ "$SERVICE_NAME" = "api-gw" ]
then
    ./bin/engine-server --name=api-gw --config=./config/config.yaml --http-address=$BIND_HTTP_ADDR --log-porf=true
elif [ "$SERVICE_NAME" = "fed" ]
then
    cat /opt/ligase/config/fed.yaml
    ./bin/federation --name=fed --config=./config/fed.yaml
elif [ "$SERVICE_NAME" = "content" ]
then
    cat /opt/ligase/config/config.yaml
    ./bin/content --config=./config/config.yaml --http-address=$HTTP_ADDR --log-porf=true
else
     echo "invalid service name $SERVICE_NAME"
fi
