FROM docker.finogeeks.club/finochat/dendrite_build
#FROM golang

RUN mkdir -p /mnt/data/logs
ENV LOG_DIR=/mnt/data/logs

COPY dockerize /usr/bin

ADD ./config /opt/federation/config
ADD ./federation /opt/federation/federation
ADD ./run.sh /opt/federation/run.sh

WORKDIR /opt/federation
#CMD ./federation
CMD ./run.sh $RUN_ENV
