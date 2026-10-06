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

Early skeleton — data model and TUI scaffolding are in place; the translate and dictionary screens are under active development. See [`specifications.md`](specifications.md) and [`pages.md`](pages.md) for the full design.

## Tech

- Go, [Bubble Tea](https://charm.land) for the TUI
- SQLite (`modernc.org/sqlite`, pure Go, no cgo) — one database file per user (GM or player)

## Running

```sh
go run ./cmd
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

Pushing a `v*` tag runs the [release workflow](.github/workflows/release.yml), which uses GoReleaser to build the 12 binaries and attach them, with a checksums file and a changelog, to a GitHub release:

```sh
git tag v0.1.0
git push origin v0.1.0
```

A tag with a suffix, like `v0.2.0-rc1`, is published as a prerelease: it is never marked as the latest release. The changelog of a stable release starts from the previous stable release, so it includes the changes of its prereleases.

The binaries aren't signed: macOS blocks them until opened with right click → Open (or `xattr -d com.apple.quarantine <file>`), and Windows SmartScreen shows a warning.
