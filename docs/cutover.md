# Cutover

## Python runtime removal checklist

- Replace the Python CLI entrypoint with the Go binary.
- Replace npm and PyPI wrappers so they invoke the Go binary.
- Remove Python runtime modules after parity workflows pass.
- Keep Markdown spec fixtures and run-artifact contract fixtures.
