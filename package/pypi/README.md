# scafld

This package installs a `scafld` console script that downloads and runs the
native Go CLI from the matching GitHub release.

```bash
pipx install scafld
scafld --version
```

The Go binary is the product. This PyPI package is only a distribution shim.

Environment overrides:

- `SCAFLD_BINARY=/path/to/scafld` runs a local binary instead of downloading.
- `SCAFLD_INSTALL_DIR=/custom/cache` controls where downloaded binaries are cached.
- `SCAFLD_INSTALL_BASE_URL=https://host/assets` downloads release assets from a mirror.
