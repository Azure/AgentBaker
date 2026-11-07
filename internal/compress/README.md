# internal/compress

`flate` and `gzip` are copies of Go's `compress/flate` and `compress/gzip` from **go1.27.3** (non-test files only).
The only change is the `compress/flate` import in `gzip/gzip.go` and `gzip/gunzip.go`, which points to the copy instead.
`LICENSE` and `PATENTS` are from the same Go release.

## Why

AgentBaker gzips customData in-process (`pkg/agent/utils.go`), and the base64-encoded result must fit in Compute's
customData limit (`MaxCustomDataLength`). The compressed bytes depend on the `compress/flate` implementation, which ships
with the Go toolchain and changed in Go 1.27. With the standard library, the same AgentBaker code produced ~1% larger
customData when built with Go 1.26 than with Go 1.27, so the size checked in CI was not the size sent by consumers that
build AgentBaker with an older Go release. With this copy, the output is the same whichever Go release builds AgentBaker.

## Consumer API and RP integration

Consumers must use `agent.GzipCustomData` from `github.com/Azure/agentbaker/pkg/agent` when re-compressing customData.
It uses the same pinned compressor and `BestCompression` as AgentBaker's internal generation, returning raw gzip bytes;
base64 encoding remains the caller's responsibility. E2E file injection uses this API as well.
The `internal/compress` packages are implementation details and cannot be imported by RP.

This change alone does not pin every payload sent by RP. RP's kube-reserved rewrite currently decompresses, patches,
and re-compresses customData using the Go standard library. A follow-up RP change must upgrade its AgentBaker dependency
and replace that compression step with `agent.GzipCustomData` in the same PR, preserving base64 encoding and patching.
Versioned AgentBakerSvc deployments must also adopt a release containing this change; older deployments retain
their original encoding. End-to-end toolchain independence applies only to paths using the shared compressor.

Go upgrades will not automatically update this copy, including compression optimizations and security fixes.
Those require reviewing and updating the pinned source and digest explicitly.

## Updating

Do not edit these files. Changing the compressor changes the customData bytes and size for every node, and
`TestGetGzippedBufferFromBytesIsPinned` in `pkg/agent/utils_test.go` fails until its digest is updated.

To copy from another Go release, run from the repository root:

```sh
GO_VERSION=go1.27.3
GOROOT_DIR="$(GOTOOLCHAIN=${GO_VERSION} go env GOROOT)"
for pkg in flate gzip; do
  rm internal/compress/${pkg}/*.go
  find "${GOROOT_DIR}/src/compress/${pkg}" -maxdepth 1 -name '*.go' ! -name '*_test.go' -exec cp {} internal/compress/${pkg}/ \;
done
cp "${GOROOT_DIR}/LICENSE" "${GOROOT_DIR}/PATENTS" internal/compress/
chmod u+w internal/compress/LICENSE internal/compress/PATENTS internal/compress/*/*.go
```

Then move the `compress/flate` import in `gzip/gzip.go` and `gzip/gunzip.go` into its own group as
`github.com/Azure/agentbaker/internal/compress/flate`, and update the Go version in this file.

To check the copy against a Go release:

```sh
for pkg in flate gzip; do
  diff -r -x '*_test.go' -x testdata "${GOROOT_DIR}/src/compress/${pkg}" internal/compress/${pkg}
done
```

Only the `compress/flate` imports in `gzip/` should differ.
