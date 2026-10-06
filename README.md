# hushtongue

A terminal (TUI) toolkit for building and playing with a conlang (constructed language) in a tabletop RPG.

Built for an isolated, ancient, peaceful island people whose language players learn gradually, in-character, over the course of a campaign.

## Concept

Two apps, one engine:

- **`langgen`** (GM) — full authority over the dictionary: manual word entry, automatic word generation, and linguistic structure (prefixes/suffixes/roots).
- **`langlearn`** (player) — a personal, partial notebook filled in by hand during play. No access to the GM's master dictionary.

Each player's notebook evolves independently and is never synced with the GM's dictionary or other players' notebooks — that's the point: everyone learns the conlang imperfectly, at their own pace, just like a real second language picked up through play.

Both apps share the same translation UI: type in either language, get live color-coded feedback per word (known / known-but-untranslated / unknown), autocomplete via an in-memory trigram index, and navigate multiple candidate translations when a word has more than one.

## Status

Work in progress. See [`specifications.md`](specifications.md) and [`pages.md`](pages.md) for the full design.

- **Dictionary screen** — done: browse and filter the words of both languages, add, rename and delete words, link translations by hand (picking an existing word or typing a new one), and navigate from a word to its translations. Renaming a word into an existing one offers to merge them. In `langgen` only: set a word's kind (root/prefix/suffix) and generate translations.
- **Translate screen** — not started yet.

## Tech

- Go, [Bubble Tea](https://charm.land) for the TUI
- SQLite (`modernc.org/sqlite`, pure Go, no cgo) — one database file per user (GM or player)

## Installing and running

Download the app from the [latest release](https://github.com/wolf29f/hushtongue/releases/latest): **`langlearn`** for players, **`langgen`** for the GM. Pick the file matching your system:

| System | File |
| --- | --- |
| Linux, x86 PC | `<app>-linux-amd64.tar.gz` |
| Linux, ARM (Raspberry Pi…) | `<app>-linux-arm64.tar.gz` |
| macOS, Apple Silicon (M1 and later) | `<app>-darwin-arm64.tar.gz` |
| macOS, Intel | `<app>-darwin-amd64.tar.gz` |
| Windows, most PCs | `<app>-windows-amd64.exe` |
| Windows, ARM (Snapdragon…) | `<app>-windows-arm64.exe` |

The apps run in a terminal: a modern one with Unicode and colors gives the best rendering.

### Linux

```sh
tar -xzf langlearn-linux-amd64.tar.gz
./langlearn
```

To launch it from anywhere, move it to a directory of your `PATH`, e.g. `mv langlearn ~/.local/bin/`.

### macOS

```sh
tar -xzf langlearn-darwin-arm64.tar.gz
xattr -d com.apple.quarantine langlearn
./langlearn
```

The binaries aren't signed, so macOS blocks them when downloaded from a browser: the `xattr` command lifts that block (it reports `No such xattr` when there is none, which is fine). Alternatively, try to launch it once, then allow it in System Settings → Privacy & Security → "Open Anyway".

### Windows

Run the `.exe` from Windows Terminal or PowerShell:

```powershell
.\langlearn-windows-amd64.exe
```

Double-clicking it also works and opens it in a console window. The binaries aren't signed, so Windows SmartScreen may warn on first launch: click "More info", then "Run anyway".

### Data

Each app stores its dictionary in a single SQLite file, and its logs next to it:

| System | Dictionary | Logs |
| --- | --- | --- |
| Linux | `~/.local/share/hushtongue/hushtongue.db` | `~/.local/state/hushtongue/app.log` |
| macOS | `~/Library/Application Support/hushtongue/hushtongue.db` | `~/Library/Application Support/hushtongue/app.log` |
| Windows | `%LOCALAPPDATA%\hushtongue\hushtongue.db` | `%LOCALAPPDATA%\hushtongue\app.log` |

Back up or move a dictionary by copying that file. `langgen` and `langlearn` use the same path: on the same computer, they share the same dictionary.

## Running from source

```sh
go run -tags gm ./cmd   # langgen, the GM app
go run ./cmd            # langlearn, the player app
```

## Building

The `gm` build tag selects the target:

- with `-tags gm`: **`langgen`**, the GM app
- without it: **`langlearn`**, the player app

```sh
go build -o dist/langgen -tags gm ./cmd
go build -o dist/langlearn ./cmd
```

Without a version, the apps show `dev` in the bottom right corner. To set one:

```sh
go build -o dist/langgen -tags gm \
  -ldflags "-X github.com/wolf29f/hushtongue/internal/config.Version=v0.1.0" ./cmd
```

Build both apps for all common targets into `dist/` with [GoReleaser](https://goreleaser.com), configured in [`.goreleaser.yaml`](.goreleaser.yaml):

```sh
goreleaser build --snapshot --clean
# or, without installing it
go run github.com/goreleaser/goreleaser/v2@latest build --snapshot --clean
```

To also produce the release files (`.tar.gz` archives, Windows `.exe` and checksums) without publishing anything:

```sh
goreleaser release --snapshot --clean
```

## Releasing

Pushing a `v*` tag runs the [release workflow](.github/workflows/release.yml), which uses GoReleaser to build both apps for the 6 targets and attach them, with a checksums file and a changelog, to a GitHub release:

```sh
git tag v0.1.0
git push origin v0.1.0
```

A tag with a suffix, like `v0.2.0-rc1`, is published as a prerelease: it is never marked as the latest release. The changelog of a stable release starts from the previous stable release, so it includes the changes of its prereleases.
