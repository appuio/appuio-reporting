FROM docker.io/library/alpine:3.17 as runtime

RUN \
  apk add --update --no-cache \
    bash \
    coreutils \
    curl \
    ca-certificates \
    tzdata

ENTRYPOINT ["appuio-cloud-reporting"]
COPY appuio-cloud-reporting /usr/bin/

USER 65536:0
