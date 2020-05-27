FROM alpine

RUN mkdir -p /mnt/data/logs
RUN mkdir -p /opt/ligase/log
#RUN apk add --update-cache ca-certificates
RUN apk add librdkafka
ENV LOG_DIR=/mnt/data/logs

ENV SERVICE_NAME=monolith

ADD ./config /opt/ligase/config
ADD ./bin /opt/ligase/bin
ADD ./start.sh /opt/ligase/start.sh

#EXPOSE 8008 8448 7000
EXPOSE 8008 8448 7000 18008 18448

WORKDIR /opt/ligase
CMD ./start.sh
