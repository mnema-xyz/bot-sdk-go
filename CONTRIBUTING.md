# Contributing

Thanks for helping. Bug reports, fixes and docs are welcome; for a new feature,
please open an issue first, since the SDK follows what the Mnema Talk server offers.

## Setup

```sh
git clone https://github.com/mnema-xyz/bot-sdk-go.git
cd bot-sdk-go
go test -race ./...
```

Go 1.26 or newer. The unit tests need no network and no server. The contract
tests against a real server are described in the README (Contract tests); CI runs
them on every pull request.

## Rules

- **Minimal dependencies.** Keep external dependencies to a minimum (currently
  only `gorilla/websocket`).
- **Limits**: Go source files at most 400 lines, test files at most 800 lines,
  functions at most 80 lines (raw line span incl. blanks and comments), cognitive
  complexity at most 15 (`gocognit`, code) and cyclomatic complexity at most 15
  (`gocyclo`, code and tests). Split code rather than raising a limit; no
  allowlists, no exceptions. CI also runs `govulncheck`.
- **Behaviour tests.** Test what a bot sees (requests, frames, events), not
  internals.
- **Commits** follow [Conventional Commits](https://www.conventionalcommits.org)
  (`fix:`, `feat:`, `docs:`, `ci:`, ...). Pull requests are squash-merged once CI is green.
- English only, in code, comments and docs.

## Releases

A release is a tag `vX.Y.Z` pushed from a commit on `main` after a pull request
that updates `CHANGELOG.md`. When the tag is pushed, the release workflow
verifies the tag is on `main`, runs the test suite, creates the GitHub release
with the changelog notes, and warms the Go module proxy.

## License

By contributing you agree that your contribution is licensed under the
[MIT License](LICENSE).
