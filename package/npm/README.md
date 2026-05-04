# scafld

**A deterministic protocol for multi-phase agent work.**
The agent passes through. The protocol stays.

Plans outlive agents. Sessions hold the receipts. Reviews take nothing on faith.

```bash
npm install -g scafld
scafld --version
```

The Go binary is the product. This npm package is only a distribution shim that
fetches the matching native binary from GitHub releases.

Environment overrides:

- `SCAFLD_BINARY=/path/to/scafld` runs a local binary instead of the downloaded one.
- `SCAFLD_SKIP_DOWNLOAD=1` skips binary download for packaging tests.
- `SCAFLD_INSTALL_BASE_URL=https://host/assets` downloads release assets from a mirror.
