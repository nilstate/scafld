# scafld

**A deterministic protocol for multi-phase agent work.**
The agent passes through. The protocol stays.

Plans outlive agents. Sessions hold the receipts. Reviews take nothing on faith.

```bash
pipx install scafld
scafld --version
```

The Go binary is the product. This PyPI package is only a distribution shim
that fetches the matching native binary from GitHub releases.

Environment overrides:

- `SCAFLD_BINARY=/path/to/scafld` runs a local binary instead of downloading.
- `SCAFLD_INSTALL_DIR=/custom/cache` controls where downloaded binaries are cached.
- `SCAFLD_INSTALL_BASE_URL=https://host/assets` downloads release assets from a mirror.
