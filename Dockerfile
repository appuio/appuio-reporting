FROM docker.io/library/alpine:3.17 as runtime

RUN \
  apk add --update --no-cache \
    bash \
    coreutils \
    curl \
    ca-certificates \
    tzdata

ENTRYPOINT ["appuio-reporting"]
COPY appuio-reporting /usr/bin/

USER 65536:0
