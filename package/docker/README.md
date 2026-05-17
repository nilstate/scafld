# OCI Image

The OCI image is built from the same tag as the GitHub release and copies the
released Linux binary into the image. It is a CI convenience channel, not a
separate runtime.

Target image names:

- `ghcr.io/nilstate/scafld:vX.Y.Z`
- `ghcr.io/nilstate/scafld:latest`

The Dockerfile carries OCI metadata labels so GHCR can connect the image to
`github.com/nilstate/scafld` and display the canonical project description and
license.
