# Host-side image fetching

`IMAGE_FETCHER_SERVER_TRANSFER=true` delegates registry fetching to the host
containerd Transfer service. AgentBaker selects this path only for ACL AMD64.
Other OS images and ACL ARM64 retain the existing client Fetch/Pull path.

Transfer uses `/etc/containerd/certs.d`, selects the requested platform, and
marks the image managed for CRI. Images below the existing 200 MiB threshold
are unpacked locally into OverlayFS without another registry traversal.
`IMAGE_FETCH_ONLY=true` skips that unpack step.

Registry-side extensions such as referrer retention belong to the host daemon,
not image-fetcher. There is no extension-specific capability probe or daemon
configuration change here. A successful Transfer alone does not prove that
an extension ran; integration tests must inspect the retained artifacts.

Transfer errors currently propagate to the caller. The ACL-specific nonfatal
referrer fallback is not implemented yet.

Run `go test ./...` and `go build ./...` in this directory.
