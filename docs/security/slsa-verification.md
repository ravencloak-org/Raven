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

```
gh attestation verify \
  oci://ghcr.io/ravencloak-org/go-api:latest \
  --owner ravencloak-org \
  --predicate-type https://slsa.dev/provenance/v1
```

**SBOM:**

```
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

```
cosign verify-attestation \
  --type slsaprovenance1 \
  --certificate-identity-regexp \
    'https://github.com/ravencloak-org/Raven/.github/workflows/docker.yml@.*' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  ghcr.io/ravencloak-org/go-api:latest
```

## What Verification Proves

- The image was built from `ravencloak-org/Raven` by the `docker.yml`
  workflow on a GitHub-hosted runner.
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
- If you verify an image pushed before this feature landed, both commands
  will fail with "no attestation found". That is expected.
- If `gh attestation verify` fails with an auth error, run `gh auth login`
  and ensure the account has read access to `ravencloak-org/Raven`.

## References

- [SLSA v1.0 specification](https://slsa.dev/spec/v1.0/)
- [GitHub artifact attestations](https://docs.github.com/en/actions/security-guides/using-artifact-attestations-to-establish-provenance-for-builds)
- [`gh attestation verify` manual](https://cli.github.com/manual/gh_attestation_verify)
- [`cosign verify-attestation` docs](https://docs.sigstore.dev/cosign/verifying/verify/)
