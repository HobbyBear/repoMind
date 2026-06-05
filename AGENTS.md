# Agent Rules

## GitHub Release Upload Checklist

When publishing a GitHub Release for this repository, do not stop after local builds. A release is complete only after the commit, tag, release page, all assets, and checksums are verified on GitHub.

### Required Steps

1. Run the full test suite before release:

   ```bash
   go test ./...
   ```

2. Commit source changes before creating the release tag.

3. Create and push the version tag that matches the release, for example `v0.6.0`.

4. Build all required platform binaries:

   - `repomind-linux-amd64`
   - `repomind-linux-arm64`
   - `repomind-darwin-amd64`
   - `repomind-darwin-arm64`
   - `repomind-windows-amd64`
   - `repomind-windows-arm64`
   - `repomind-windows-amd64.exe`
   - `repomind-windows-arm64.exe`

5. Do not upload compressed packages such as `.tar.gz` or `.zip` by default. The release assets should be the bare binaries above plus `SHA256SUMS`, unless the user explicitly asks for archive packages.

6. Generate `SHA256SUMS` only after every final binary asset exists. If any asset is added or replaced later, regenerate `SHA256SUMS` and re-upload it with `--clobber`.

7. Upload all required binary assets to the GitHub Release. Use `--clobber` when intentionally replacing an existing asset.

8. Verify the online release through GitHub API or `gh release view`, not only by looking at the browser UI:

   ```bash
   gh release view <version> --repo HobbyBear/repoMind --json assets --jq '.assets[].name' | sort
   ```

9. Compare the online asset list with the required asset list above. The release is incomplete if any required filename is missing.

10. Verify direct download URLs for assets that are easy to miss, especially Windows binaries without `.exe`:

   ```bash
   curl -L -sS -o /dev/null -w "%{http_code}\n" \
     https://github.com/HobbyBear/repoMind/releases/download/<version>/repomind-windows-amd64

   curl -L -sS -o /dev/null -w "%{http_code}\n" \
     https://github.com/HobbyBear/repoMind/releases/download/<version>/repomind-windows-arm64
   ```

   Both must return `200`.

### Important Notes

- The macOS and Linux installer downloads binaries without archive extraction, so bare binaries must be uploaded.
- Windows release assets must include both `.exe` and no-suffix binary names.
- Compressed packages are not required for normal releases and should not be uploaded unless explicitly requested.
- GitHub's web UI can lag or collapse asset rows. Treat the GitHub API, `gh release view`, and direct download URL checks as the source of truth.
- Do not say the release is complete until online assets and direct download URLs have been verified.
