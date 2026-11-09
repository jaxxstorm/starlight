# Development and releases

## Preview the documentation

The documentation uses [Docsify](https://docsify.js.org/) with no build step. Install [mise](https://mise.jdx.dev/), then from the repository root:

```sh
mise trust
mise install
mise run serve
```

Open <http://localhost:8000>. Serve the directory over HTTP rather than opening `index.html` directly. Docsify, search, and syntax-highlighting assets load from a CDN, so the preview requires internet access unless those assets are cached. To publish, serve the contents of `Docs/` with a static host; `.nojekyll` preserves the sidebar on GitHub Pages deployments.

## Tests

Run Go tests from the repository root:

```sh
go test ./...
go test -race ./...
go vet ./...
```

Package and release-tool tests:

```sh
python3 -m unittest discover -s deployment/openwrt -p 'test_*.py'
python3 -m unittest discover -s tools/release -p 'test_*.py'
```

The optional native package smoke test requires `STARLIGHT_TEST_BINARY` to point to a native build. Upstream interoperability tests have their own prerequisites; consult `satmux/upstream_test.go` for their environment and skip conditions.

## Build tooling

```sh
python3 tools/release/build.py native --command ./cmd/starlight
python3 tools/release/build.py matrix --command ./cmd/starlight
python3 tools/release/validate.py
```

The provisional matrix uses CGO-disabled Linux amd64 (GOAMD64=v1), arm64 (GOARM64=v8.0), and ARMv7 builds. Output defaults to `dist`; existing output files are not overwritten intentionally. Matrix compilation is not a device support guarantee. The validator runs tests, native role-help smoke checks, matrix builds, ELF/build-setting checks, and development checksum verification.

## Artifact helpers

Given the three correctly named binaries, such as `starlight_v1.2.3_linux_amd64`, in a directory:

```sh
python3 tools/release/artifacts.py create path/to/artifacts v1.2.3
python3 tools/release/artifacts.py verify path/to/artifacts v1.2.3
python3 tools/release/tag.py v1.2.3
```

The tag helper validates a semantic version; it does not publish or verify Git state. Checksums detect corruption but are not an authenticated release channel on their own.

Version/revision reporting, automated publication, and a signed OpenWrt feed remain unfinished. Existing tools are development tooling, not a complete release pipeline.

## OpenSpec

`openspec/config.yaml` contains the project context. Active changes under `openspec/changes/` track protocol work, routing, persistent installation, the ground service, and releases. Design/evidence documents include deployment history; some task checklists lag the implementation. Use these as engineering context, not end-user installation instructions.
