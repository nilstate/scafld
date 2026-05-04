# OCI Image

The OCI image should be built from the same tag as the GitHub release and should
copy the released Linux binary into the image. It is a CI convenience channel,
not a separate runtime.

Target image names:

- `ghcr.io/nilstate/scafld:vX.Y.Z`
- `ghcr.io/nilstate/scafld:latest`

Suggested base image:

```Dockerfile
FROM alpine:3.22
RUN apk add --no-cache ca-certificates git bash
COPY scafld /usr/local/bin/scafld
ENTRYPOINT ["scafld"]
```
