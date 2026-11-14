# hushtongue

A toolkit for building and playing with a conlang (constructed language) in a tabletop RPG.

Built for an isolated, ancient, peaceful island people whose language players learn gradually, in-character, over the course of a campaign.

## Status

hushtongue v2, a self-hosted web app (Go server, React front, Postgres), is in development on `main`. Its design is in [`SPEC-V2.md`](SPEC-V2.md).

The v1 terminal apps, `hushtongue-gm` and `hushtongue-player`, live on the [`v1` branch](https://github.com/wolf29f/hushtongue/tree/v1) and in the [v1 releases](https://github.com/wolf29f/hushtongue/releases). Their « Exporter le dictionnaire » menu entry, from `v1.1.0`, writes the file v2 imports.

## Concept

Two roles, one engine:

- **GM** — full authority over their dictionary: manual word entry, automatic word generation, and linguistic structure (prefixes/suffixes/roots).
- **Player** — a personal, partial notebook filled in by hand during play. No access to the GM's master dictionary.

Each player's notebook evolves independently and is never synced with the GM's dictionary or other players' notebooks — that's the point: everyone learns the conlang imperfectly, at their own pace, just like a real second language picked up through play.

|                                       | GM  | Player |
| ------------------------------------- | --- | ------ |
| Translate, in both directions         | ✅   | ✅      |
| Browse and edit the dictionary        | ✅   | ✅      |
| Add and delete translations by hand   | ✅   | ✅      |
| Copy the translation to the clipboard | ✅   | ✅      |
| Word kinds (root/prefix/suffix)       | ✅   | ❌      |
| Translation generation                | ✅   | ❌      |

Both roles default to the language the user knows the least: the source language for the GM, the conlang for players. This applies to the translation direction and to the dictionary's word list.

## Translation generation (GM)

- Every decomposition of the source word into known prefixes, a root and known suffixes is listed (`anti-voiture-ette`), the whole word as root included.
- Each decomposition is combined with the translations of each part, one proposal per combination: complete existing translations first, then by number of translated parts, pure generation last.
- A part without translation gets a generated conlang word: open syllables, soft consonants, 2-3 syllables for a root and 1 for an affix, sometimes 2. Generation is deterministic — a given source word always gives the same result — unless that result is already taken by a conlang word of the same kind, in which case a variant is used.
- Applying a proposal creates the missing words and links. With several parts, the source word is also linked to the concatenated conlang word, stored as a root. When that root already translates other source words, the proposal is flagged as their homonym; applying it anyway creates the homonym.

## Design

- Words are deduplicated on their normalized form (lowercased, accents stripped), per language and kind.
- Several translations of a word are shown in insertion order, without ranking.
- Autocomplete uses an in-memory trigram index built from the user's dictionary: at a TTRPG's scale (a few thousand words), it isn't worth a database-side index.

## Running from source

```sh
DATABASE_URL=postgres://… HUSHTONGUE_GM_PASSWORD=… go run ./cmd/server
```

The server reads its configuration from the environment and refuses to start, listing every missing or invalid variable, when it is incomplete:

| Variable                       | Required | Default                   |
| ------------------------------ | -------- | ------------------------- |
| `DATABASE_URL`                 | yes      |                           |
| `HUSHTONGUE_GM_PASSWORD`       | yes      |                           |
| `HUSHTONGUE_GM_USERNAME`       | no       | `mj`                      |
| `HUSHTONGUE_PUBLIC_URL`        | no       | deduced from each request |
| `HUSHTONGUE_ADDR`              | no       | `:8080`                   |
| `HUSHTONGUE_SOURCE_LANG_LABEL` | no       | `Français`                |
| `HUSHTONGUE_CON_LANG_LABEL`    | no       | `conLang`                 |

It logs JSON to the standard output and, on SIGINT or SIGTERM, lets the requests in progress finish before stopping. At startup, it applies the pending database migrations. `GET /api/health/live` answers as soon as it runs, `GET /api/health/ready` only when the database answers too; `server healthcheck` exits with 0 when the server listening on `HUSHTONGUE_ADDR` is ready, 1 otherwise.

At startup, it also creates the GM's account, or renames it after `HUSHTONGUE_GM_USERNAME`, keeping its dictionary. The GM logs in with `HUSHTONGUE_GM_USERNAME` and `HUSHTONGUE_GM_PASSWORD`, at most 5 attempts per minute and per IP. Behind a reverse proxy, the client's IP is the last entry of `X-Forwarded-For`: the server must not be reachable without the proxy, which would let a client pick its IP. The session cookie is `Secure` unless `HUSHTONGUE_PUBLIC_URL` starts with `http://`, which local development over http needs (`HUSHTONGUE_PUBLIC_URL=http://localhost:8080`).

The server runs as a single instance: the login attempts are counted in memory, and so will be the cache of each user's lexicon. Running several instances would need the attempts in the database and the lexicon reloaded on each request.

## Tests

```sh
go test ./...         # everything, the Postgres tests in throwaway containers
go test -short ./...  # without Docker, the Postgres tests skipped
```

The Postgres tests start one container per package with [testcontainers](https://golang.testcontainers.org), each test in its own schema. Without `-short`, they fail when Docker isn't running rather than being skipped. With [Colima](https://github.com/abiosoft/colima), point testcontainers at its socket:

```sh
export DOCKER_HOST=unix://$HOME/.colima/default/docker.sock
export TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE=/var/run/docker.sock
```
