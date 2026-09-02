# Main release dry-run success — 2026-09-02

- Run: `33543469369` — <https://github.com/ming-kang/BUPT_EC/actions/runs/33543469369>
- Pushed HEAD: `40f0b46381bba66e93e45f8ab98a5a874c53a0c2` (`main` push, not a tag).
- Result: **success**. `quality-gate / quality`, `build-go (amd64)`, `build-go (arm64)`, and `release` all completed successfully. The release job ran composition, provenance attestation, and **Upload dry-run release assets**; tag-only notes/publication steps were skipped.
- Downloaded `release-assets-dry-run` artifact (ID `9814617331`, 8,098,072 B; retained) to a temporary directory and removed it after verification.
- Artifact contains exactly: `bupt-ec-linux-amd64.tar.gz`, `bupt-ec-linux-arm64.tar.gz`, `checksums.txt`, and `install.sh`. Both checksum entries verified.
- Both tarballs contain exactly their root directory plus `.env.example`, `README.md`, `bupt-ec`, `bupt-ec-cli`, and `install.sh`. Packaged and top-level Installers match the generated repository artifact; the two packaged CLI files are byte-identical and each has exactly one `CLI_BUILD_VERSION="main-40f0b46"` marker. Each packaged Go binary contains the same `main-40f0b46` injected marker once.
- Post-run remote inspection found no `v0.3.0` tag/release. GitHub still lists `v0.2.0` as Latest and retains the existing nightly prerelease/tag as required until post-stable-release cleanup. No release was published by this run.
- GitHub emitted non-blocking Node 20 deprecation and cache-service warnings; all jobs nevertheless succeeded.

Real Linux E2E remains deferred and blocks `scripts/release.sh v0.3.0`, stable tag publication, and nightly cleanup.
