# Verifying SLSA Build Provenance for Raven Container Images

Every container image published by this repository to GHCR carries two
cryptographic attestations:

- **SLSA v1 build provenance** — who built the image, from which commit,
  on which workflow, at what time.
- **SPDX SBOM** — the full software bill of materials for the image.

Both are signed keylessly via [Sigstore](https://www.sigstore.dev/) using
GitHub OIDC and anchored in the public [Rekor](https://docs.sigstore.dev/logging/overview/)
transparency log. Attestations are stored both in the GitHub attestation
store and as OCI referrers alongside the image in GHCR.

Images covered: `ghcr.io/ravencloak-org/{go-api,python-worker,frontend}`.
Attestations exist for every image pushed from `main` or a `v*.*.*` tag on
or after the commit that landed this feature.

## Verify with `gh` (GitHub CLI)

Primary path — no extra install beyond `gh`.

**Provenance:**

```bash
gh attestation verify \
  oci://ghcr.io/ravencloak-org/go-api:latest \
  --owner ravencloak-org \
  --predicate-type https://slsa.dev/provenance/v1
```

**SBOM:**

```bash
gh attestation verify \
  oci://ghcr.io/ravencloak-org/go-api:latest \
  --owner ravencloak-org \
  --predicate-type https://spdx.dev/Document/v2.3
```

Substitute `go-api` with `python-worker` or `frontend` for the other two
images, and replace `latest` with any tag or `sha256:` digest you want to
verify.

## Verify with `cosign`

For environments without `gh`:

**Provenance:**

```bash
cosign verify-attestation \
  --type slsaprovenance1 \
  --certificate-identity-regexp \
    'https://github.com/ravencloak-org/Raven/.github/workflows/(docker|release)\.yml@.*' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  ghcr.io/ravencloak-org/go-api:latest
```

**SBOM:**

```bash
cosign verify-attestation \
  --type spdxjson \
  --certificate-identity-regexp \
    'https://github.com/ravencloak-org/Raven/.github/workflows/(docker|release)\.yml@.*' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  ghcr.io/ravencloak-org/go-api:latest
```

## Always Verify by Digest in Production

Tags like `:latest` are mutable. For any real supply-chain check, pin to
the image's immutable `sha256:` digest. Resolve a tag to a digest with
either of:

```bash
crane digest ghcr.io/ravencloak-org/go-api:latest
```

```bash
docker buildx imagetools inspect ghcr.io/ravencloak-org/go-api:latest --format '{{json .Manifest.Digest}}'
```

Then verify the digest directly:

```bash
gh attestation verify \
  oci://ghcr.io/ravencloak-org/go-api@sha256:<digest> \
  --owner ravencloak-org \
  --predicate-type https://slsa.dev/provenance/v1
```

`cosign verify-attestation` accepts the same `<image>@sha256:<digest>`
form.

## What Verification Proves

- The image was built from `ravencloak-org/Raven` by either the
  `docker.yml` workflow (main-branch pushes) or the `release.yml`
  workflow (tag releases), on a GitHub-hosted runner.
- The commit SHA in the provenance matches a real commit on this repo.
- The image digest you pull is byte-for-byte the one that was attested.
- The SBOM was produced by the same build.

## What Verification Does NOT Prove

- That the image is free of known vulnerabilities — that is what Trivy and
  `govulncheck` in `.github/workflows/security.yml` cover.
- That the build is bit-for-bit reproducible.
- Runtime integrity. Attestation is a build-time guarantee.

## Notes

- `actions/attest-sbom@v3` supports SPDX only; the SBOM attestation uses
  `spdx-json`. A CycloneDX variant is not produced.
- If you verify an image pushed before this feature landed (PR
  [#352](https://github.com/ravencloak-org/Raven/pull/352)), both commands
  will fail with "no attestation found". That is expected.
- If `gh attestation verify` fails with an auth error, run `gh auth login`
  and ensure the account has read access to `ravencloak-org/Raven`.
- Re-pushing an image with an unchanged digest will add a new referrer
  manifest rather than dedupe. `gh attestation verify` picks the first
  valid attestation, so this is benign.
- The SBOM is attached to the multi-arch **manifest-list digest**, so a
  single SBOM covers all platforms for a given tag. Syft runs against one
  platform when generating it, so platform-specific contents (e.g. arm64
  vs. amd64 system libs) may not be fully enumerated. Per-platform SBOM
  attestation is a tracked follow-up.

## References

- [SLSA v1.0 specification](https://slsa.dev/spec/v1.0/)
- [GitHub artifact attestations](https://docs.github.com/en/actions/security-guides/using-artifact-attestations-to-establish-provenance-for-builds)
- [`gh attestation verify` manual](https://cli.github.com/manual/gh_attestation_verify)
- [`cosign verify-attestation` docs](https://docs.sigstore.dev/cosign/verifying/verify/)
