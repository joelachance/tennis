<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset=".github/assets/tennis-logo-dark.png">
    <img src=".github/assets/tennis-logo-light.png" alt="Tennis" width="226">
  </picture>
</p>

<p align="center">
  <a href="https://github.com/satoricorp/tennis/actions/workflows/ci.yml"><img src="https://img.shields.io/endpoint?url=https://raw.githubusercontent.com/satoricorp/tennis/badges/tests.json" alt="Tests passing"></a>
  <a href="https://github.com/satoricorp/tennis/actions/workflows/ci.yml"><img src="https://github.com/satoricorp/tennis/actions/workflows/ci.yml/badge.svg?branch=main" alt="CI"></a>
  <a href="https://github.com/satoricorp/tennis/releases"><img src="https://img.shields.io/github/v/release/satoricorp/tennis" alt="Latest release"></a>
  <a href="https://pkg.go.dev/github.com/satoricorp/tennis"><img src="https://pkg.go.dev/badge/github.com/satoricorp/tennis.svg" alt="Go reference"></a>
  <img src="https://img.shields.io/github/go-mod/go-version/satoricorp/tennis" alt="Go version">
  <a href="LICENSE"><img src="https://img.shields.io/github/license/satoricorp/tennis" alt="License"></a>
</p>

Tennis is a fully local (offline) CLI that indexes and searches your context. Tennis works for most data types, as well as ChatGPT & Claude chats. Tennis embeds and stores everything in both SQLite and in markdown files called 'cards'.

Every search runs a keyword search (BM25, through SQLite's FTS5) and a semantic search, then fuses both into one score. Tennis uses `potion-retrieval-32M`, a static table of vectors rather than a neural network. That pairing is what makes the setup work offline.

# Getting Started

## Install (MacOS & Linux)

**Option 1: the installer** (needs curl and nothing else)

```bash
curl -fsSL https://raw.githubusercontent.com/satoricorp/tennis/main/scripts/install.sh | sh
```

It picks the archive for your OS and architecture, checks it against the SHA-256 that release was published with, and installs it to `~/.local/bin`. If that folder isn't on your `PATH` yet, it tells you the line to add.

To pin a version, or install somewhere else:

```bash
curl -fsSL https://raw.githubusercontent.com/satoricorp/tennis/main/scripts/install.sh | TENNIS_VERSION=0.2.0 sh
TENNIS_INSTALL_DIR=/usr/local/bin sh scripts/install.sh
```

**Option 2: Using Go** (needs Go 1.25 or newer)

```bash
go install github.com/satoricorp/tennis/cmd/tennis@latest
```

This puts `tennis` in `$(go env GOPATH)/bin`. That folder must be on your `PATH`.

**Option 3: Build from source**

```bash
git clone https://github.com/satoricorp/tennis
cd tennis
CGO_ENABLED=0 go build -o tennis ./cmd/tennis
```

NOTE: `CGO_ENABLED=0` makes one self-contained binary with no C libraries.

```bash
tennis version
```

NOTE:
The first time you index something, Tennis downloads its embedding model (about 123MB) into `~/.cache/tennis`. It also uses the network to summarize cards when `ANTHROPIC_API_KEY` or `OPENAI_API_KEY` is set, and for every `add` and `search` in an `--openai` namespace. You can skip these features by not setting those variables.

To read PDFs on Linux, also install `pdftotext` (for example `sudo apt install poppler-utils`). On a Mac, Tennis defaults to `pdftotext`, with the Spotlight importer as a fallback.

# Examples

## Basic Example

Index a folder of notes:

```bash
$ tennis add ~/Documents/notes
tennis: created namespace "context" bound to builtin:potion-retrieval-32M
tennis: summarizing cards with anthropic:claude-opus-5
tennis: /Users/you/Documents/notes: reading plain files
imported 3, skipped 0 unchanged, 3 chunks in "context", 3 cards in /Users/you/tennis
```

What happened:

- Tennis created a **namespace** called `context`. A namespace is a collection of context, and `context` is the default one.
- It read the 3 files, split them into chunks, and stored them in `~/.tennis/db.sqlite`.
- It wrote one markdown summary **card** per file into `~/tennis`. These are human readable markdown files you can explore to understand what you've indexed without any tools.

Run search:

```bash
$ tennis search "do we set a session cookie"
> # Session handling

  Make the login flow remember the user between sessions. The session cookie is
  set with an expiry so the browser keeps it after the tab closes.

  auth.md [2026-10-08] 0.0328
```

You get the best match, printed in full. The last line includes the file name, date the file was edited, and a confidence score.

Ask for more results with `-k`:

````bash
$ tennis search "tell me about our auth flow" -k 2
 1. auth.md [2026-10-08] 0.0328  [kw#1 sem#1]
    # Session handling

    Make the login flow remember the user between sessions. The session cookie
    is set with an expiry so the browser keeps it after the tab closes.

 2. retry.md [2026-10-08] 0.0161  [sem#2]
    # Retries

    Failed requests back off exponentially, doubling the delay each attempt up
    to a cap of thirty seconds.


Running `add` multiple times is non-destructive. Tennis skips any files that 1. it already knows about and 2. haven't changed:

```bash
$ tennis add ~/Documents/notes
tennis: summarizing cards with anthropic:claude-opus-5
tennis: /Users/you/Documents/notes: reading plain files
imported 0, skipped 3 unchanged, 0 chunks in "context"
````

List what is stored:

```bash
$ tennis ls
DATE             SOURCE         DOCS  TITLE
2026-10-08 14:09 file              1  auth.md
2026-10-08 14:09 file              1  config.md
2026-10-08 14:09 file              1  retry.md

3 files
```

## What to use Tennis for

### Working offline

Tennis runs on your machine and never makes network calls.

### Searching files and chat history locally

It finds exact terms (names, IDs, error codes) and paraphrases in the same search.

### Free, local knowledge base

Write documents in and query them back through the Go library, a local HTTP API, or the CLI with `--json`.

### Speed

On an Apple M4, a search over about 117,000 chunks (74,000 chat messages) takes about 2 seconds.

## What Tennis isn't as good at

### Millions of documents

Tennis is not optimized for massive document stores, and is ideal for individual power users.

### A shared server

Tennis doesn't have any authentication features, and should stay sandboxed on a single machine.

### Chinese, Japanese, and Korean languages

The keyword index can't split those languages into words, however, semantic search should work. Follow along here: [issue #1](https://github.com/satoricorp/tennis/issues/1).

### Images, audio, and video

Tennis indexes text only.

### Windows

Only macOS and Linux are supported.

## Full Example

```bash
# 1. Import AI chat history. Tennis detects the format from the contents.
tennis add ~/.claude                            # Claude Code sessions on this machine
tennis add ~/.codex                             # Codex sessions on this machine
tennis add ~/Downloads/chatgpt-export.zip       # a ChatGPT data export
tennis add ~/Downloads/claude-export.zip        # a Claude.ai data export

# 2. Keep work documents in their own namespace.
tennis add ~/work/handbook --ns work

# 3. See what is stored, newest first.
tennis ls
tennis ls chatgpt                               # only one source
tennis ls --ns work

# 4. Search, and narrow the search with attribute filters.
tennis search "token refresh" -k 5
tennis search "token refresh" --where project=tennis,branch=main
tennis search "retry logic" --where role=user   # only things you typed
tennis search "vacation policy" --ns work

# 5. Choose one kind of search.
tennis search "ERR_CONN_RESET" --mode keyword   # exact words only
tennis search "the app forgets me" --mode semantic

# 6. Read a whole conversation: give a filter and no query.
tennis search --where session=7f3a2c10

# 7. Get JSON for scripts.
tennis search "token refresh" -k 5 --json

# 8. Delete one document, then a whole namespace.
tennis rm ~/work/handbook/old-policy.md --ns work
tennis ns rm work
```

## Filters

Every document has attributes you can filter with `--where`. Join comparisons with commas (all must match): `=`, `!=`, `>`, `>=`, `<`, `<=`. Quote a value that contains a comma: `--where 'path="/Users/you/Budget, 2026.md"'`.

| Documents             | Attributes                                                                                 |
| --------------------- | ------------------------------------------------------------------------------------------ |
| Files                 | `path`, `name`, `size`, `modified`                                                         |
| Chat messages         | `source`, `session`, `role`, `title`, `created`                                            |
| Claude Code and Codex | also `project` (the repo the session started in), `cwd`, and `worktree` when there was one |
| Claude Code           | also `branch`; a subagent's turns have `subagent`                                          |

## Cards

Every file/chat gets its own markdown card. With `ANTHROPIC_API_KEY` or `OPENAI_API_KEY` set, the card contains a short summary otherwise its the header of the document.

| Setting   | Default                    | Change with                                                    |
| --------- | -------------------------- | -------------------------------------------------------------- |
| Format    | detected from the contents | `--chatgpt`, `--claude`, `--claude-code`, `--codex`, `--files` |
| Database  | `~/.tennis/db.sqlite`      | `--db` or `$TENNIS_DB`                                         |
| Namespace | `context`                  | `--ns` or `$TENNIS_NS`                                         |
| Cards     | `~/tennis`                 | `--cards <dir>` or `--no-cards`                                |

Run `tennis --help` for the main commands and `tennis --agents` for every command.

# Embedding tennis with your application

There are three ways to use Tennis from your own code.

**1. Go library.** This is the same code as [`examples/basic`](examples/basic/main.go):

```go
import "github.com/satoricorp/tennis"

db, err := tennis.Open("~/.tennis/db.sqlite")
if err != nil {
    log.Fatal(err)
}
defer db.Close()

// Create a namespace once. Later, open it with db.Namespace(ctx, "agents").
ns, err := db.CreateNamespace(ctx, "agents", tennis.NamespaceOptions{})

// Write documents. Writing an existing ID replaces that document.
_, err = ns.Write(ctx, []tennis.Document{
    {ID: "a1", Text: "make the login flow remember the user between sessions",
        Attributes: map[string]any{"status": "merged", "cost": 4}},
})

// Search.
results, err := ns.Query(ctx, tennis.Query{
    Text:   "keep me signed in",
    TopK:   5,                             // default 10
    Mode:   tennis.Hybrid,                 // or tennis.Keyword, tennis.Semantic
    Filter: tennis.Eq("status", "merged"), // also NotEq, In, Gt, Gte, Lt, Lte, Glob, And, Or
})
for _, r := range results {
    fmt.Println(r.ID, r.Score, r.KeywordRank, r.SemanticRank, r.Text)
}
```

`Namespace` also has `Delete`, `Get`, `List`, and `Count`. The [API reference](https://pkg.go.dev/github.com/satoricorp/tennis) lists everything.

**2. Local HTTP API.** For any other language. Start the server:

```bash
tennis serve                        # listens on 127.0.0.1:8817
```

| Method | Path                         | Request body                                    | Response                                                      |
| ------ | ---------------------------- | ----------------------------------------------- | ------------------------------------------------------------- |
| `GET`  | `/health`                    | none                                            | `{"status": "ok", "version": ..., "db": ...}`                 |
| `GET`  | `/v1/namespaces`             | none                                            | list of namespaces with counts, or `null` when there are none |
| `POST` | `/v1/namespaces/{ns}/write`  | `{"documents": [{"id", "text", "attributes"}]}` | `{"written", "skipped", "chunks"}`                            |
| `POST` | `/v1/namespaces/{ns}/query`  | `{"text", "top_k", "mode", "where"}`            | `{"results": [...]}`                                          |
| `POST` | `/v1/namespaces/{ns}/delete` | `{"ids": [...]}`                                | `{"deleted": n}`                                              |

Namespaces are created on the first write. `where` uses the same syntax as the CLI's `--where`.

```bash
curl -s localhost:8817/v1/namespaces/notes/write -d '{"documents": [{"id": "a1", "text": "make the login flow remember the user"}]}'
curl -s localhost:8817/v1/namespaces/notes/query -d '{"text": "keep me signed in", "top_k": 5}'
```

# Supported file types

| What                             | Files                                                                    |
| -------------------------------- | ------------------------------------------------------------------------ |
| Plain text                       | any file whose contents are text: `.md`, `.txt`, `.go`, `.json`, a `Makefile`, … |
| Office                           | `.docx`, `.pptx`, `.xlsx`                                                |
| Web pages and e-books            | `.html`, `.htm`, `.xhtml`, `.epub`                                       |
| PDF                              | `.pdf`                                                                   |
| Rich text and older Word         | `.rtf`, `.rtfd`, `.doc`, `.odt`, `.webarchive`                           |
| Apple iWork, older Office, email | `.pages`, `.numbers`, `.key`, `.xls`, `.ppt`, `.ods`, `.odp`, `.eml`, `.emlx` |
| ChatGPT and Claude.ai history    | a data export, as a `.zip` or unzipped folder, including project files   |
| Claude Code history              | `~/.claude`, including subagent transcripts                              |
| Codex history                    | `~/.codex`                                                               |
| Program output                   | newline-delimited JSON on stdin, with `add --ndjson`                     |

# Contributing

You need Go 1.25 or newer, on macOS or Linux.

1. Clone the repository and download the model the tests use. Without it, about 50 tests are skipped.

   ```bash
   git clone https://github.com/satoricorp/tennis
   cd tennis
   scripts/fetch-test-model.sh
   ```

2. Make your change. Ideally, you run the CI checks locally:

   ```bash
   go vet ./...
   test -z "$(gofmt -l $(git ls-files '*.go'))"
   go test ./...
   CGO_ENABLED=0 go build -o tennis ./cmd/tennis
   ```

Questions and bugs go in [an issue](https://github.com/satoricorp/tennis/issues).

# Thanks

- [Minish Lab](https://github.com/MinishLab/model2vec), for model2vec and the models that make embeddings possible.
- [SQLite](https://sqlite.org) and its [FTS5](https://sqlite.org/fts5.html) extension for keyword search.
- [sugarme/tokenizer](https://github.com/sugarme/tokenizer), for tokenizers in Go.
- [modernc.org/sqlite](https://gitlab.com/cznic/sqlite), for SQLite in Go.
