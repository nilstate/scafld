# Architecture

scafld uses a hexagonal layout with a hard import rule.

```text
cmd/scafld -> internal/adapters/cli -> internal/app -> internal/core
```

`internal/core` contains pure domain code. It imports only the standard library
and other core packages. It does not know about Markdown, JSON files, process
execution, Git, providers, terminal output, or the filesystem.

`internal/app` contains use cases. Each use case owns the narrow port interfaces
it needs in its own package. There is no shared `internal/ports` package because
that tends to turn into a broad service registry.

`internal/adapters` contains concrete IO. Non-CLI adapters satisfy app ports
implicitly and do not import app packages. The CLI adapter is the composition
root and the only adapter allowed to wire app use cases to concrete adapters.

`internal/platform` contains small infrastructure primitives such as atomic file
writes and signal handling. Platform code imports only the standard library and
must not encode scafld product policy.

Import-boundary tests in `internal/arch` are release-blocking.
