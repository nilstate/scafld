# scafld

This package installs the native `scafld` Go CLI from the matching GitHub
release and exposes it as an npm `bin`.

```bash
npm install -g scafld
scafld --version
```

The Go binary is the product. This npm package is only a distribution shim.

Environment overrides:

- `SCAFLD_BINARY=/path/to/scafld` runs a local binary instead of the downloaded one.
- `SCAFLD_SKIP_DOWNLOAD=1` skips binary download for packaging tests.
- `SCAFLD_INSTALL_BASE_URL=https://host/assets` downloads release assets from a mirror.
