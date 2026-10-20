# hushtongue

A terminal (TUI) toolkit for building and playing with a conlang (constructed language) in a tabletop RPG.

Built for an isolated, ancient, peaceful island people whose language players learn gradually, in-character, over the course of a campaign.

## Concept

Two apps, one engine:

- **`langgen`** (GM) — full authority over the dictionary: manual word entry, automatic word generation, and linguistic structure (prefixes/suffixes/roots).
- **`langlearn`** (player) — a personal, partial notebook filled in by hand during play. No access to the GM's master dictionary.

Each player's notebook evolves independently and is never synced with the GM's dictionary or other players' notebooks — that's the point: everyone learns the conlang imperfectly, at their own pace, just like a real second language picked up through play.

|                                                  | `langgen` (GM) | `langlearn` (player) |
| ------------------------------------------------ | -------------- | -------------------- |
| Translate, in both directions                    | ✅              | ✅                    |
| Browse and edit the dictionary                   | ✅              | ✅                    |
| Add and delete translations by hand              | ✅              | ✅                    |
| Copy the translation to the clipboard            | ✅              | ✅                    |
| Word kinds (root/prefix/suffix)                  | ✅              | ❌                    |
| Translation generation                           | ✅              | ❌                    |

Both apps default to the language the user knows the least: the source language for the GM, the conlang for players. This applies to the translation direction and to the dictionary's word list.

## Screens

### Translate

- A button switches the direction; switching clears the input, after a confirmation when it isn't empty.
- The text is typed word by word, a word being committed on space or enter. Each word is colored as it is typed: green when it has a translation, orange when it is known but not translated yet, red when it is unknown.
- Up to 5 completions of the word being typed are listed (followed by `…` when there are more), ranked by trigram similarity. Up/down browse them, going back past the first one returns to the typed word.
- Tab/shift+tab move the focus between the input, the translation choice and the buttons.
- In the translation choice, each word shows its translations stacked in a column: left/right change word, up/down change translation. The last item of each column is a `+` adding a translation through the translation picker; `d` deletes the selected translation link.
- A button stores the unknown words of the input in the dictionary.
- `ctrl+y` copies the translated text, each word replaced by its selected translation; `ctrl+v` pastes a text to translate.

### Dictionary

- Lists the words of one language; a button switches the language, another adds a word. `/` filters the list.
- Enter opens a word's sheet, which lists its translations (enter on a translation opens that word's sheet) and lets you:
  - add a translation, picking an existing word of the other language or typing a new one (stored as a root);
  - delete a translation link, the target word being kept;
  - rename the word, translation links being kept. Renaming it into an existing word offers to merge them: their translations are grouped and the existing word's spelling is kept;
  - delete the word and its translation links, after a confirmation;
  - `langgen` only: change the word's kind, which deletes its translation links (after a confirmation when it has some), and generate a translation for a source word.

### Translation generation (`langgen`)

- Every decomposition of the source word into known prefixes, a root and known suffixes is listed (`anti-voiture-ette`), the whole word as root included.
- Each decomposition is combined with the translations of each part, one proposal per combination: complete existing translations first, then by number of translated parts, pure generation last. Existing parts are green, generated ones red.
- A part without translation gets a generated conlang word: open syllables, soft consonants, 2-3 syllables for a root and 1 for an affix. Generation is deterministic — a given source word always gives the same result — unless that result is already taken by a conlang word of the same kind, in which case a variant is used.
- Applying a proposal creates the missing words and links. With several parts, the source word is also linked to the concatenated conlang word, stored as a root.

## Design

- Go, [Bubble Tea](https://charm.land) for the TUI. A single engine (`internal/tui`) is shared by both apps: the `gm` build tag sets `config.IsForGM`, which gates the GM-only features. Word kinds and generation share that one flag, since generating words without being able to set their kind makes no sense.
- SQLite with `modernc.org/sqlite` (pure Go, no cgo, so cross-compiling needs no C toolchain), one database file per user. Two tables: `words` (language, text, normalized form, kind) and `translations`, a many-to-many link between a source word and a conlang word.
- Words are deduplicated on their normalized form (lowercased, accents stripped), per language and kind.
- Several translations of a word are shown in insertion order, without ranking.
- Linking only words of different languages is checked in Go, SQLite having no cross-row constraint without a trigger.
- Autocomplete uses an in-memory trigram index built when the translation page loads the dictionary, rather than SQLite FTS5: at a TTRPG's scale (a few thousand words), it isn't worth a virtual table and its sync triggers.

FTS5 can be reconsidered if the lexicon ever grows to tens of thousands of words.

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
