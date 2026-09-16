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
