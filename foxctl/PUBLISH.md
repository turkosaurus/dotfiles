# Publishing foxctl

Three artifacts, three distribution stories. The extension is the awkward
one because Firefox refuses to install unsigned extensions on the release
channel.

## 1. Extension — Mozilla signing is the gate

| Route | What it means | Good for |
|---|---|---|
| **AMO listed** | Public listing on addons.mozilla.org. Human review (days–weeks). Users install via the store. | Real publishing. Deferred. |
| **AMO unlisted (self-hosted)** | Mozilla signs the `.xpi` but doesn't list it. We distribute the file ourselves (GitHub release). Automated review, usually minutes. | **v1 plan.** |
| **Temporary add-on** | Drag a folder into `about:debugging`. Wiped on restart. | Development only. |
| **Disable signing** | Only works in Developer Edition / Nightly / ESR with `xpinstall.signatures.required=false`. Refuses on release Firefox. | Personal/dev fallback. |

Unlisted-signed is the sweet spot: permanent install on stock Firefox, no
store listing, no review wait. `web-ext sign --channel=unlisted` in CI
returns a signed `.xpi` that gets attached to the GitHub release. The
extension ID is fixed at first signing; the native-messaging manifest
pins it.

The shipped manifest lists **both** the dev ID (a UUID chosen in
`manifest.json` for `web-ext run`) and the signed AMO ID, so dev and
release coexist on one machine.

## 2. Native host — ordinary binary + a manifest file

No signing gate. Two things land on disk:

- The binary, anywhere on `$PATH` (default `/usr/local/bin/foxctl-host`
  or `~/.local/bin/foxctl-host`).
- A JSON manifest at an OS-specific path that tells Firefox where the
  binary is and which extension IDs may launch it:
  - **macOS:** `~/Library/Application Support/Mozilla/NativeMessagingHosts/foxctl.json`
  - **Linux:** `~/.mozilla/native-messaging-hosts/foxctl.json`
  - **Windows (v2):** a registry key under `HKCU\Software\Mozilla\NativeMessagingHosts\foxctl`

## 3. CLI — easiest

Same Go binary or a sibling of the host. Drop on `$PATH`. Nothing special.

## Install path (v1)

Primary install is a shell installer pulled from a GitHub release:

```
curl -fsSL https://github.com/user/foxctl/releases/latest/download/install.sh | bash
```

The installer:

1. Picks the right tarball for the user's OS + arch.
2. Drops `foxctl` and `foxctl-host` somewhere on `$PATH` (configurable).
3. Writes the native-messaging manifest at the OS-specific path, pointed
   at the installed `foxctl-host`.
4. Prints a one-line instruction to download the `.xpi` from the same
   release and install it via `about:addons → Install Add-on From File`.
5. Suggests `foxctl doctor` as the next step.

Two steps because the browser side has to be a user gesture; an installer
can't auto-load extensions.

Packaging (Homebrew tap, Arch AUR, Nix flake, etc.) is a "later, when
asked" — `install.sh` covers macOS and Linux without preassuming a
package manager.

## Release pipeline

Single GitHub release per tag, containing:

- `foxctl-<version>-<os>-<arch>.tar.gz` for darwin/arm64, darwin/amd64,
  linux/amd64, linux/arm64 (built with `goreleaser`).
- `foxctl-<version>.xpi` (signed via `web-ext sign --channel=unlisted`).
- `install.sh` (versioned and content-addressed; `latest/download/`
  always points at the freshest one).
- `SHA256SUMS`.

CI (GitHub Actions) on tag push:

- `goreleaser release` — cross-compiles, packages, attaches binaries.
- `web-ext sign` — signs the extension with AMO API credentials in
  secrets, uploads the `.xpi`.
- Posts the release URL to wherever release notifications go.

## Versioning and compatibility

- Extension, host, and CLI are released as one tag. They share the wire
  protocol; version skew means breakage.
- Wire protocol carries a `version` field; the host refuses connections
  outside its supported range and tells the user which side is stale
  (exit 5).
- The extension auto-updates from AMO; the host updates only when the
  user reruns `install.sh`. `foxctl doctor` surfaces drift.

## Uninstall

Documented one-liner:

```
foxctl uninstall          # removes manifest, binaries, log dir
```

Plus a sentence in the README: "remove the foxctl extension via
`about:addons` to clear the browser side."

## Checklist for the first release

- [ ] AMO developer account, extension ID reserved, first `web-ext sign`
      run successful.
- [ ] `web-ext sign` wired into GitHub Actions with API credentials in
      secrets.
- [ ] `goreleaser` config building host + CLI for the four target
      `GOOS/GOARCH` combos.
- [ ] `install.sh` tested on a clean macOS and a clean Linux box.
- [ ] README with: `install.sh` one-liner, `.xpi` install instructions,
      `foxctl doctor` callout, privacy note (extension reads every tab
      URL/title in the profile).
- [ ] Uninstall path verified (`foxctl uninstall` + extension removal).
