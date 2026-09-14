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
- `kind` on `words` prepares the linguistic structure (root / prefix / suffix) without automatic recognition complexity — see Out of scope.
- No FTS5: at this scale (a few hundred to a few thousand words), trigram autocomplete lives **in memory on the Go side**, not in the database. FTS5 on SQLite requires a separate virtual table + sync triggers (no equivalent to Postgres's `pg_trgm`) — unjustified cost here.

## SQLite Driver

`modernc.org/sqlite` — pure Go (ccgo transpilation of the C amalgamation), zero cgo, FTS5 compiled but unused (see above). Cross-compile for macOS/Linux/Windows as straightforward as today, without a C toolchain per target.

## Software Architecture

A single TUI engine (`internal/tui`) shared between both binaries, parameterized by a capabilities set:

```go
type Capabilities struct {
    AllowAuthoring bool // auto-generation of missing words + root/prefix/suffix declaration
    AllowClipboard bool // copy translated text
}
```

|                                                              | `langgen` (GM) | `langlearn` (player) |
| ------------------------------------------------------------ | -------------- | -------------------- |
| Translate (both directions)                                  | ✅              | ✅                    |
| Stacked candidates + reordering                              | ✅              | ✅                    |
| Add manual translation                                       | ✅              | ✅                    |
| Delete a translation                                         | ✅              | ✅                    |
| Trigram autocomplete (Tab)                                   | ✅              | ✅                    |
| `AllowAuthoring` (auto-generation + root/prefix/suffix kind) | ✅              | ❌                    |
| `AllowClipboard` (copy translated text)                      | ✅              | ✅                    |

`AllowAuthoring` groups auto-generation and `kind` editing into a single flag rather than two, because one without the other makes no functional sense: generating a word without being able to type it as root/prefix/suffix prevents building structure on top of it.

## UI Behaviors

### Input and Coloring
- Input retokenized in real time: each word gets a green background (known) or orange background (unknown) as the user types.
- Custom input component (bubbletea sub-`Model`), not the base `bubbles` `textinput` — necessary to intercept each `KeyMsg` and recolor on the fly.

### Autocomplete (Tab)
- Trigram index built in memory at dictionary load time (a few hundred to a few thousand words → negligible).
- Scoring by trigram overlap between the input and known words, sorted by descending score.
- `Tab` cycles through the proposed candidates for the word currently being typed.

### Multiple Candidates (many-to-many)
- For a word with multiple translations, candidates are displayed stacked vertically on that slot.
- Navigation:
  - **up/down**: changes the selected candidate for the current slot
  - **left/right**: changes slot (next/previous word in the phrase)
  - **shift+left/right** (or equivalent): moves the current slot within the sequence of the reconstructed phrase — useful when word order differs between the source language and the conlang.
- Unknown words (orange, `?`): action to add a translation.
- Known word: action to delete a translation from the current slot.

### Export
- Copy translated text to clipboard (`AllowClipboard`, granted to both profiles). Already identified dependency: `atotto/clipboard` (transitive via `bubbles/textinput`, to be promoted to a direct dependency).

## Out of Scope for This Version (noted for later, not forgotten)

- **Automatic affix recognition**: detecting that an unknown word contains a known prefix/suffix and proposing a partial translation. The `kind` field on `words` prepares the ground (prefixes/suffixes are already declared and typed), but the decomposition algorithm is not built in this version.
- **Dynamic candidate ranking**: if a translation priority mechanism is needed in real usage, add `is_primary` rather than reintroducing a full ranking system.
- **FTS5**: to be reconsidered only if the lexicon grows well beyond the expected scale for a TTRPG (tens of thousands of words) — not anticipated.

## Open Points to Settle During Implementation

- Exact distinction between "candidate" navigation (up/down) and "reorder phrase" (shift+left/right): confirmed in principle, to be validated once the component is built and tested in real usage — the exact ergonomics (keys, visual feedback of the move) remain to be refined in practice.
- The `source_word_id` constraint must reference `lang='source'` (and inversely for `con_word_id`): not enforced in SQL (SQLite does not support cross-row CHECK constraints without a dedicated trigger), to be validated on the Go application side.
