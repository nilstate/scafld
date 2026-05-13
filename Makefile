GOFILES := $(shell find . -path './.tools' -prune -o -name '*.go' -print)

.PHONY: fmt vet test race arch package-check package-manager-check release-snapshot check

fmt:
	@test -z "$$(gofmt -l $(GOFILES))"

vet:
	@go vet ./...

test:
	@go test ./...

race:
	@go test -race ./...

arch:
	@go test ./internal/arch -run 'ImportBoundaries|CoreIsPure|CoreTransitiveDepsAreStdlib|AppDoesNotImportAdapters|PortsAreUseCaseOwned|PortsAreNarrow|ProviderBoundary|CLIIsThin'

package-check:
	@test -x bin/scafld
	@sh -n bin/scafld
	@node --check package/npm/bin/scafld.js
	@node --check package/npm/lib/install.js
	@node --check package/npm/lib/platform.js
	@cd package/npm && npm pack --dry-run --ignore-scripts >/dev/null
	@python3 -m compileall -q package/pypi/src
	@bash -n scripts/render-package-managers.sh scripts/publish-homebrew.sh scripts/publish-scoop.sh scripts/prepare-winget-submission.sh scripts/audit-scafld-metadata.sh

package-manager-check:
	@tmp="$$(mktemp -d)"; trap 'rm -rf "$$tmp"' EXIT; CHECKSUMS_FILE=package/testdata/checksums.txt OUT_DIR="$$tmp" scripts/render-package-managers.sh 0.0.0 >/dev/null

release-snapshot:
	@scripts/build-release-artifacts.sh 0.0.0-snapshot

check: fmt vet arch package-check package-manager-check test race
