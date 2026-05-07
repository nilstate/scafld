# syntax=docker/dockerfile:1.7

FROM alpine:3.22

ARG VERSION
ARG TARGETARCH

RUN apk add --no-cache ca-certificates git

ADD --chmod=0755 "https://github.com/nilstate/scafld/releases/download/v${VERSION}/scafld_${VERSION}_linux_${TARGETARCH}" /usr/local/bin/scafld

ENTRYPOINT ["scafld"]
