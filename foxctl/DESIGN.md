# foxctl — Firefox tab/group CRUD from the shell

CLI control of Firefox's native tab groups (FF 139+): create, name, color,
populate, close. Declarative "open this set" / "close this set" workflows.
Sets are portable files (yaml/toml/json) or embedded blocks in GitHub
issues/PRs.

## The three pieces and who launches whom

Firefox will not take orders from a shell. The only place tab-group CRUD can
happen is JavaScript running inside a WebExtension. So we need a bridge.

```
   ┌──────────────────┐   native messaging   ┌──────────────────┐
   │  WebExtension    │ ◄──── stdio ────►    │  Native host     │
   │  (ext/)          │   (4-byte+JSON)      │  (host/, Go)     │
   │                  │                      │                  │
   │  bg.js           │                      │  spawned by FF   │
   │  popup.html      │                      │  long-lived      │
   │  browser.tabs.*  │                      │  listens on UDS  │
   │  browser.tabGroups.*                    │  mux by req id   │
   └──────────────────┘                      └────────▲─────────┘
                                                      │ UDS
                                                      │ JSON-RPC
                                              ┌───────┴────────┐
                                              │  foxctl CLI    │
                                              │  (cmd/foxctl)  │
                                              └────────────────┘
                                                      ▲
                                                      │
                                              ┌───────┴────────┐
                                              │  sets/*.yaml   │
                                              │  gh issues/PRs │
                                              └────────────────┘
```

- **Firefox spawns the host.** First time the extension calls
  `connectNative("foxctl")`, Firefox reads a manifest off disk that points
  at the host binary, launches it as a child process, and pipes JSON over
  its stdin/stdout. The host lives for the life of the browser session.
- **The CLI does not start anything.** It connects to a Unix domain socket
  the host opens at startup, sends one JSON request, prints the reply, exits.
- **The host is a two-headed translator.** Socket request → forward to
  extension via stdout → extension calls `browser.tabGroups.*` → reply
  comes back via stdin → host writes it to the socket. Concurrent requests
  are muxed by `id`.

Why this shape:

- Extensions are sandboxed; they can't open server sockets. Native messaging
  is Mozilla's escape hatch.
- The host has to be the process Firefox launched (that's how the stdio
  pipes get wired). The CLI's job is just to find it via the socket.

## End-to-end flow

```
$ foxctl group create reading --color blue --url a --url b --url c
        │
        ▼ (Unix socket)
   native host (running, spawned by Firefox earlier)
        │
        ▼ (stdin/stdout, length-prefixed JSON)
   extension background script
        │
        ▼
   browser.tabs.create({url, discarded:true}) × 3
   browser.tabs.group({tabIds:[…]})
   browser.tabGroups.update({title:"reading", color:"blue"})
        │
        ▼ (reply travels home)
   {"groupId": 42}
```

## Repo layout

```
foxctl/
├── DESIGN.md
├── PUBLISH.md
├── ext/                       WebExtension (MV2 for Firefox)
│   ├── manifest.json          perms: tabs, tabGroups, nativeMessaging
│   ├── bg.js                  native port + dispatch table
│   ├── popup.html             two-button popup UI
│   └── popup.js               snapshot / open-set handlers
├── host/                      Go native-messaging host
│   ├── main.go                stdio framing + UDS server + req-id mux
│   ├── proto.go               request/response types
│   └── manifest/foxctl.json   installed to native-messaging-hosts/
├── cmd/foxctl/                Go CLI (cobra)
│   └── main.go
├── sets/                      user-defined named tab sets
│   └── example.yaml
└── install.sh                 builds, drops manifest, points at host binary
```

## Extension UI (deliberately tiny)

Toolbar icon opens a popup with exactly two actions:

- **Snapshot active group →** prompts a Save dialog, writes the focused
  tab's group as yaml to the chosen path.
- **Open set…** file picker, runs the same code path as `foxctl apply`.

No tab list, no color picker, no settings page. Everything else is the
CLI. The popup exists because "name + color a group I just built up" is
genuinely fiddly at the shell, and "what group am I even in right now" is
the one question the CLI can't answer ergonomically.

## Wire protocol

One JSON object per request/response. Same shape on both legs so the host
is a near-transparent bridge.

```json
// request
{"id":"abc","op":"group.create","args":{"name":"reading","color":"blue","urls":["…"]}}

// response
{"id":"abc","ok":true,"data":{"groupId":42}}
{"id":"abc","ok":false,"error":{"code":"not_found","message":"no such group: reading"}}
```

Native messaging adds a 4-byte little-endian length prefix per message
(Mozilla spec). UDS uses newline-delimited JSON. Every message carries a
`version` field; mismatched majors → exit 5.

Error codes: `not_found`, `ambiguous`, `invalid_arg`, `firefox_busy`,
`permission_denied`, `version_skew`, `internal`.

## CLI specification

### Commands

```
foxctl group list   [--window=ID]                            [--json]
foxctl group show   <name|id>                                [--json]
foxctl group create <name>  [--color=C] [--url=U]... [--eager] [--window=ID]
foxctl group update <name|id> [--name=NEW] [--color=C] [--collapsed|--expanded]
foxctl group close  <name|id>
foxctl group add    <name|id> --url=U...                     [--eager]

foxctl tab list  [--group=NAME] [--window=ID]                [--json]
foxctl tab open  <url> [--group=NAME]                        [--eager]
foxctl tab close <id>

foxctl apply    <source>          [--replace] [--eager] [--window=ID]
foxctl snapshot <name|id>         [--output=PATH] [--format=yaml|toml|json]
foxctl push     <name|id>         --issue=org/repo#NNNN

foxctl doctor                                                [--json]
foxctl logs                       [--follow] [--host|--extension|--both]
foxctl version
```

### `<source>` for `apply`

| Form                       | Meaning                                                       |
|----------------------------|---------------------------------------------------------------|
| `./path/to/set.yaml`       | local file; format by extension (`.yaml`/`.yml`/`.toml`/`.json`) |
| `-`                        | stdin; `--format` required                                    |
| `gh:org/repo#1234`         | fetch issue/PR via `gh`, extract fenced `foxctl:set` block    |
| `https://gist.github.com/…`| fetch raw, extract block if HTML, parse if raw                |

### Global flags

```
--profile=NAME       Firefox profile (default: only running one; error if ambiguous)
--socket=PATH        override socket discovery
--json               machine-readable output (where supported)
-v, --verbose        diagnostic output to stderr
```

### Locked behaviors

- **Tab loading:** default `discarded:true` (lazy). `--eager` to fully load.
- **`apply` on existing group with same name:** merge — set title/color/
  collapsed, append missing URLs at end, leave existing tabs alone.
  `--replace` closes existing tabs in the group first, then opens fresh.
- **Dedup:** none. Duplicate URLs across windows/groups are allowed; if a
  URL is already open elsewhere, `apply` opens another copy.
- **Color values:** `grey | blue | red | yellow | green | pink | purple | cyan`
  (Firefox's eight built-ins). Invalid color → exit 4.
- **Name resolution:** group titles are not unique in Firefox.
  `<name|id>` resolves: if numeric → id; else title match. Multiple
  matches → exit 2 with the candidate list. `--id=N` to disambiguate.

### Set file format

```yaml
name: reading           # required
color: blue             # optional; one of the eight Firefox colors
collapsed: false        # optional; default false
urls:                   # required, non-empty
  - https://news.ycombinator.com
  - https://lobste.rs
```

TOML and JSON are isomorphic to the above.

### GitHub issue/PR embedding

`foxctl push <group> --issue=org/repo#1234` writes (or replaces) this
block in the issue body via `gh issue edit`:

````markdown
<!-- foxctl:set -->
```yaml
name: reading
color: blue
urls:
  - https://news.ycombinator.com
  - https://lobste.rs
```
<!-- /foxctl:set -->
````

`foxctl apply gh:org/repo#1234` reads the block back out. Round-trip
preserves the rest of the issue body untouched.

### Exit codes

| Code | Meaning |
|------|---------|
| 0    | success |
| 2    | not found (group, set source, issue) or ambiguous name |
| 3    | Firefox not reachable / host not running / extension not loaded |
| 4    | invalid argument or set file |
| 5    | version skew between CLI / host / extension |

### Output

- Default: short human format (`group#42  reading (blue)  3 tabs`).
- `--json`: object or array, stable schema. Pipe-safe.
- All diagnostic output goes to stderr; stdout is for results only.

## `foxctl doctor`

Single command, one screen of output. Checks:

- Firefox process running (which versions, which profiles).
- Native messaging manifest present at the expected OS path, pointing at
  an existing binary.
- Extension loaded and connected to host (round-trip ping).
- Host, extension, CLI versions agree.
- Socket path resolved and reachable.

Each check prints `ok` / `warn` / `fail` with a one-line remediation hint.

## Logging and debugging

Native messaging owns the host's stdio; anything written to stdout that
isn't a framed message kills the connection. So:

- **Host:** all logs go to a file
  (`~/Library/Logs/foxctl/host.log` on macOS,
  `$XDG_STATE_HOME/foxctl/host.log` on Linux), never stdout.
- **Extension:** logs go to the browser console *and* are forwarded to
  the host log via a `log` op so `foxctl logs` shows both sides.
- **`foxctl logs --follow`** tails the host log.

## Security

- Native messaging host is launched only by Firefox; no network listener on
  that leg.
- UDS at `$XDG_RUNTIME_DIR/foxctl.sock` (Linux) or
  `$TMPDIR/foxctl.sock` (macOS), mode `0600`. Local user only.
- No HTTP, no remote. Wrap with `ssh -L` if remoting is ever wanted.

## Privacy footprint

The extension can read every tab's URL and title in every window of the
profile it's installed in. This is documented prominently in the README
and the AMO listing. The host writes URLs only to:

- The local log (rotated, user-readable only).
- Files the user explicitly targets via `snapshot` / `push`.
- The socket, in response to CLI requests.

Nothing is sent over the network by foxctl itself.

## Concurrency

The host accepts overlapping requests. Each incoming CLI connection gets
a unique request id; the host writes to the extension and parks the
caller on a channel keyed by that id. Replies are demultiplexed by id.
The extension is single-threaded JS, but `browser.*` calls return
promises and interleave naturally. CLIs never block each other.

## Testing strategy

- **Host:** Go unit tests; framing, UDS protocol, request mux.
- **Set parser:** table-driven tests across yaml/toml/json and the
  GitHub-embedded form.
- **CLI:** golden-file tests for human + `--json` output.
- **Extension + host integration:** `web-ext run` launches Firefox with
  the extension as a temporary add-on against a known profile; CI runs
  the CLI through a scripted set of ops and asserts on results.

## Platforms

v1: macOS + Linux (Unix domain sockets).
v2: Windows (named pipes — separate code path in the host).

## Milestones

1. Extension + host: `group.list`, `group.create`, `group.close`
   end-to-end (lazy tabs, single window).
2. CLI for those three ops, plus `doctor` and `version`.
3. Remaining ops; set parser (yaml/toml/json); `apply` (merge + replace);
   `snapshot`.
4. Extension popup; `push` / `apply gh:…` round-trip.
5. `install.sh`, README, signed `.xpi` from CI.

## Deferred (not v1)

- Set composition (`includes: [...]`).
- Windows support.
- AMO public listing (unlisted-signed is enough to start).
- macOS codesigning of the host binary (revisit if Gatekeeper complains).
