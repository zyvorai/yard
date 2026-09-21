# Upgrade

A git tag is the upgrade unit. The tag workflow builds `yard`, `yard-simulator`, and `yard-agent-gateway`, writes SHA256 checksums, and attaches `go version -m` output as `sbom.txt`.

Schema migrations run when the process starts. The new binary applies every pending version in `schema_migrations` before it serves traffic. `/readyz` fails while the database is behind the binary. Take a backup before you replace the process.

Image signing uses cosign keyless on the tag workflow. The signer is the GitHub Actions OIDC identity. Verify with `cosign verify-blob --bundle SHA256SUMS.bundle --certificate-identity-regexp 'https://github.com/zyvorai/yard/.github/workflows/release.yml' --certificate-oidc-issuer https://token.actions.githubusercontent.com SHA256SUMS`. No signing key is stored in this repository. Multi-arch images are not published; the tag workflow builds `linux/amd64` and `linux/arm64` binaries instead.
