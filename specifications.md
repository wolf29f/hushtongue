# Langue de l'Île — Specification

## Context

Conlang for a TTRPG (ancient, peaceful people, isolated island). Two interactive CLIs sharing the same interaction engine:

- **`langgen`** — GM: full authority over the dictionary, automatic word generation, linguistic structure (prefixes/suffixes).
- **`langlearn`** — player: individual and partial notebook, filled in manually during the session. No access to the master dictionary.

Core game mechanic: each player learns the language **independently and imperfectly**. The player notebook is never synchronized with the master dictionary nor with other players' notebooks — this is a feature, not a technical limitation.

## Data Model (SQLite, one file per user — GM or player)

```sql
CREATE TABLE words (
  id          INTEGER PRIMARY KEY,
  lang        TEXT NOT NULL CHECK (lang IN ('source', 'con')),
  text        TEXT NOT NULL,
  normalized  TEXT NOT NULL,             -- lowercase, accent-stripped: dedup key
  kind        TEXT NOT NULL DEFAULT 'root' CHECK (kind IN ('root', 'prefix', 'suffix')),
  created_at  TEXT NOT NULL DEFAULT (datetime('now')),
  UNIQUE(lang, normalized, kind)
);

CREATE TABLE translations (
  id              INTEGER PRIMARY KEY,
  source_word_id  INTEGER NOT NULL REFERENCES words(id) ON DELETE CASCADE,
  con_word_id     INTEGER NOT NULL REFERENCES words(id) ON DELETE CASCADE,
  source          TEXT NOT NULL DEFAULT 'manual' CHECK (source IN ('manual', 'generated')),
  created_at      TEXT NOT NULL DEFAULT (datetime('now')),
  UNIQUE(source_word_id, con_word_id)
);

CREATE INDEX idx_translations_source ON translations(source_word_id);
CREATE INDEX idx_translations_con    ON translations(con_word_id);
```

Settled decisions:
- Native many-to-many via `translations` — a source word can have multiple conlang translations (and vice versa).
- No `rank` column — display order for stacked candidates follows insertion order (`ORDER BY translations.id`). If a priority mechanism is needed later, add a simple `is_primary BOOLEAN` rather than a full ranking system.
- `kind` on `words` carries the linguistic structure (root / prefix / suffix), which translation generation uses to decompose words.
- The `source_word_id` constraint must reference `lang='source'` (and inversely for `con_word_id`): not enforced in SQL (SQLite does not support cross-row CHECK constraints without a dedicated trigger), validated on the Go side when linking or merging words.
- No FTS5: at this scale (a few hundred to a few thousand words), trigram autocomplete lives **in memory on the Go side**, not in the database. FTS5 on SQLite requires a separate virtual table + sync triggers (no equivalent to Postgres's `pg_trgm`) — unjustified cost here.

## SQLite Driver

`modernc.org/sqlite` — pure Go (ccgo transpilation of the C amalgamation), zero cgo, FTS5 compiled but unused (see above). Cross-compile for macOS/Linux/Windows as straightforward as today, without a C toolchain per target.

## Software Architecture

A single TUI engine (`internal/tui`) shared between both binaries. The `gm` build tag selects the binary: with it, `config.IsForGM` is true and the build is `langgen`; without it, the build is `langlearn`. GM-only features check `config.IsForGM`.

|                                                              | `langgen` (GM) | `langlearn` (player) |
| ------------------------------------------------------------ | -------------- | -------------------- |
| Translate (both directions)                                  | ✅              | ✅                    |
| Stacked candidates                                           | ✅              | ✅                    |
| Add manual translation                                       | ✅              | ✅                    |
| Delete a translation                                         | ✅              | ✅                    |
| Trigram autocomplete (up/down)                               | ✅              | ✅                    |
| Translation generation + root/prefix/suffix kind (`IsForGM`) | ✅              | ❌                    |
| Copy translated text to clipboard                            | ✅              | ✅                    |

Generation and `kind` editing are both gated by `IsForGM` rather than two separate flags, because one without the other makes no functional sense: generating a word without being able to type it as root/prefix/suffix prevents building structure on top of it.

Both apps default to the language the user knows the least: the conlang for players, the source language for the GM. This applies to the dictionary's word list and to the translation direction.

## UI Behaviors

### Direction
- Defaults to conlang → source for players, source → conlang for the GM.
- A button switches the direction. Switching clears the input, after a confirmation modal when the input isn't empty.

### Input and Coloring
- Input retokenized in real time, each word colored as the user types: green (has a translation), orange (known word, not translated yet), red (unknown word).
- Custom input component (bubbletea sub-`Model`), not the base `bubbles` `textinput` — necessary to intercept each `KeyMsg` and recolor on the fly.

### Focus
- `Tab`/`Shift+Tab` move the focus between the input, the translation choice and the direction button, as on the other pages. Moving from the input to the translation choice switches from input mode to translation choice mode.

### Autocomplete (up/down)
- Trigram index built in memory at dictionary load time (a few hundred to a few thousand words → negligible).
- Scoring by trigram overlap between the input and known words, sorted by descending score.
- Up to 5 candidates are shown for the word being typed, followed by `…` when there are more. Up/down browse them, and going back past the first one returns to the current input.

### Multiple Candidates (many-to-many)
- For a word with multiple translations, candidates are displayed stacked vertically on that slot.
- Navigation:
  - **up/down**: changes the selected candidate for the current slot
  - **left/right**: changes slot (next/previous word in the phrase)
- Unknown words (red, `?`): action to add a translation.
- Known word: action to delete a translation from the current slot.

### Translation Generation (`IsForGM`)
- For a source word, every decomposition into known prefixes, a root and known suffixes is listed, the whole word as root included (`translation.Propose`).
- Each decomposition is combined with the possible translations of each part, one proposal per combination: complete existing translations first, then by number of translated parts, pure generation last.
- A part without translation gets a generated conlang word (`generator.Generate`): deterministic, seeded by the normalized source word and its kind; on collision with an existing conlang word of the same kind, the seed moves to a variant.
- Applying a proposal creates the missing words and `generated` links; with several parts, the source word is also linked to the concatenated conlang word, stored as a root.

### Export
- Copy translated text to clipboard, in both apps. Already identified dependency: `atotto/clipboard` (transitive via `bubbles/textinput`, to be promoted to a direct dependency).

## Out of Scope for This Version (noted for later, not forgotten)

- **Dynamic candidate ranking**: if a translation priority mechanism is needed in real usage, add `is_primary` rather than reintroducing a full ranking system.
- **FTS5**: to be reconsidered only if the lexicon grows well beyond the expected scale for a TTRPG (tens of thousands of words) — not anticipated.
