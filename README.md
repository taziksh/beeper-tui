# beeper-tui

A keyboard-driven terminal UI for [Beeper](https://beeper.com), built on top of the local Beeper Desktop API.

> **Status:** under construction. v1 (read-only triage) is in progress. See [the v1 design spec](docs/superpowers/specs/2026-05-17-beeper-tui-design.md).

## Requirements

- Beeper Desktop running locally with the Developer API enabled (Settings → Developers → Beeper Desktop API). Requires Beeper Desktop v4.1.169+.
- Go 1.22 or later (for `go install`).

## Run From Source

During development, prefer `make run`; it always executes the current checkout
instead of an older installed binary.

```bash
make run
```

To refresh the installed `beeper-tui` command:

```bash
make install
```

## Configuration

The TUI auto-discovers your access token from a locally-running Beeper Desktop.

For headless use, set the token explicitly:

```bash
export BEEPER_ACCESS_TOKEN=<token>
```

To override the API base URL (rare):

```bash
export BEEPER_API_BASE_URL=http://localhost:23373
```

## Roadmap

- **v1** — read-only triage (chat list, debounced preview, reading mode)
- **v1.1** — search across chats and messages
- **v2** — send text messages
- **v3** — attachments, reactions, replies, threads, edits, deletes

## License

MIT. See [LICENSE](LICENSE).
