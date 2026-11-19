# github.com/mnema-xyz/bot-sdk-go

[![Go Reference](https://pkg.go.dev/badge/github.com/mnema-xyz/bot-sdk-go.svg)](https://pkg.go.dev/github.com/mnema-xyz/bot-sdk-go)
[![CI](https://github.com/mnema-xyz/bot-sdk-go/actions/workflows/ci.yml/badge.svg)](https://github.com/mnema-xyz/bot-sdk-go/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue)](LICENSE)

The official Go SDK for [Mnema Talk](https://mnema.xyz) bots: the REST API with
its rate limits, the gateway with heartbeats, resume and reconnects, slash
commands, interactions, message components, modals and the add link.

- Go 1.26 or newer, its only external dependency is
  [gorilla/websocket](https://github.com/gorilla/websocket).
- Strongly typed wire models, JSON representations, builders
  for components and modals.
- Non-blocking event dispatch with configurable handler concurrency and panic
  recovery.
- Tested on every change against a real Mnema Talk server (contract tests).

**Docs:** [developer guide](https://mnema.xyz/developers/) ·
[Go SDK reference](https://mnema.xyz/developers/go-sdk/) ·
[bot platform reference](https://github.com/mnema-xyz/talk/blob/main/doc/bots.md) ·
[Mnemo, a complete bot built on it](https://github.com/mnema-xyz/talk-mnemo-bot)

## Install

```sh
go get github.com/mnema-xyz/bot-sdk-go@latest
```

It needs Go 1.26 or newer. The package name is `mnema`.

### Moving from github.com/msch128/mnema-talk/sdk/go

Moving from the earlier monorepo location `github.com/msch128/mnema-talk/sdk/go`:
replace the dependency with `github.com/mnema-xyz/bot-sdk-go` and update your
imports from `"github.com/msch128/mnema-talk/sdk/go"` to
`"github.com/mnema-xyz/bot-sdk-go"`. The API is the same. Old pseudo-versions
continue to work from the Go module proxy.

## Quick start

Create a bot and a token first ([Getting started](https://github.com/mnema-xyz/talk/blob/main/doc/bots.md#getting-started)).

```go
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"

	mnema "github.com/mnema-xyz/bot-sdk-go"
)

func main() {
	c, err := mnema.New(mnema.Options{
		BaseURL: "https://talk.example.org",
		Token:   os.Getenv("MNEMA_TOKEN"),
		Intents: mnema.IntentGuilds | mnema.IntentGuildMessages,
		Logger:  slog.Default(),
	})
	if err != nil {
		slog.Error("configure", "err", err)
		os.Exit(1)
	}
	c.OnReady(func(ctx context.Context, r *mnema.Ready) {
		slog.Info("ready", "as", r.User.Username)
	})
	c.OnMessageCreate(func(ctx context.Context, m *mnema.Message) {
		if m.Content == "!ping" {
			_, _ = c.REST.CreateMessage(ctx, m.ChannelID, mnema.MessageCreate{
				Content:          "pong",
				MessageReference: &mnema.MessageReference{MessageID: m.ID},
			})
		}
	})
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := c.Run(ctx); err != nil {
		slog.Error("run", "err", err) // a *mnema.CloseError for 4004, 4013, 4014
		os.Exit(1)
	}
}
```

Three runnable examples in [`examples/`](examples) read `MNEMA_URL` and `MNEMA_TOKEN`:

| Example | What it does |
|---|---|
| [examples/echo](examples/echo/main.go) | repeats messages that start with `!echo ` as a reply |
| [examples/slash](examples/slash/main.go) | registers `/ping` (ephemeral reply) and `/roll` with options (deferred reply, then edited) |
| [examples/buttons](examples/buttons/main.go) | answers `!deploy` with buttons and a select, updates the message on use, opens a modal and answers its submit |

```sh
MNEMA_URL=https://talk.example.org MNEMA_TOKEN=mnb_... go run ./examples/slash
```

## Configuration

```go
mnema.New(mnema.Options{
	BaseURL:            "https://talk.example.org",
	Token:              os.Getenv("MNEMA_TOKEN"),
	Intents:            mnema.IntentGuilds | mnema.IntentGuildMessages,
	Presence:           &mnema.Presence{Status: mnema.StatusOnline},
	Logger:             slog.Default(),
	HTTPClient:         nil, // default client with 60 s timeout
	MaxRetries:         3,
	GatewayURL:         "",  // override GET /gateway address
	HandlerConcurrency: 64,
})
```

- `BaseURL`: the server address, e.g. `https://talk.example.org`.
- `Token`: the bot token (`mnb_...`).
- `Intents`: bitwise OR of `IntentGuilds` (1), `IntentGuildMembers` (2),
  `IntentGuildMessages` (512), `IntentGuildMessageReactions` (1024). Interactions
  need none. Asking for more than the admin allowed closes with `4014`.
- `Presence`: initial status and activity sent in IDENTIFY. Activity types are
  `ActivityPlaying` (0), `ActivityListening` (2), `ActivityWatching` (3),
  `ActivityCustom` (4), `ActivityCompeting` (5).
- `Logger`: `*slog.Logger`, nil defaults to `slog.Default()`.
- `HTTPClient`: `*http.Client`, nil uses a default client with a 60-second
  timeout that does not follow redirects: a redirect answer fails the call
  with a `*mnema.RESTError` carrying its status, so the bot token goes to
  `BaseURL` only. A client you pass keeps its own redirect policy (Go's
  default follows redirects and keeps `Authorization` on the same host, also
  from `https` to `http`); set `CheckRedirect` on it if you need the same
  guarantee. Attachment downloads never follow redirects.
- `MaxRetries`: bounds how often a request answered with 429 is retried (0 means 3).
- `GatewayURL`: overrides the gateway address returned by `GET /gateway`.
- `HandlerConcurrency`: bounds concurrent dispatch handler goroutines (0 means 64).

## Handlers

Register handlers before `Run`; several per event are allowed and run in order.

| Method | Dispatch | Intent |
|---|---|---|
| `OnReady(func(ctx, *Ready))` | `READY` | none |
| `OnResumed(func(ctx))` | `RESUMED` | none |
| `OnChannelsUpdate(func(ctx, *ChannelsUpdate))` | `CHANNELS_UPDATE` | `IntentGuilds` |
| `OnMemberAdd(func(ctx, *MemberAdd))` | `GUILD_MEMBER_ADD` | `IntentGuildMembers` |
| `OnMessageCreate`, `OnMessageUpdate(func(ctx, *Message))` | `MESSAGE_CREATE`, `MESSAGE_UPDATE` | `IntentGuildMessages` |
| `OnMessageDelete(func(ctx, *MessageDelete))` | `MESSAGE_DELETE` | `IntentGuildMessages` |
| `OnReactionsUpdate(func(ctx, *ReactionsUpdate))` | `MESSAGE_REACTIONS_UPDATE` | `IntentGuildMessageReactions` |
| `OnInteraction(func(ctx, *Interaction))` | `INTERACTION_CREATE` | none |
| `OnDispatch(func(ctx, name, json.RawMessage))` | every dispatch, also unknown ones | |

**Concurrency.** Each dispatch runs its handlers in a goroutine of its own, so a
slow handler never stalls heartbeats or the reading of further events. At most
`Options.HandlerConcurrency` (default 64) dispatches are handled at once; further
ones wait for a free slot without holding up the connection, up to 16 times that
many. Beyond that the client drops events with a logged warning (never READY,
RESUMED or an interaction, which always get their turn; more than 2048
interactions in flight end the connection, and the resume sends the next one
again) rather than stop reading the connection (which would leave heartbeat ACKs
unread and end in a reconnect that replays the same flood): a handler that waits
on something slow should hand the work off and return. Events can therefore be
handled concurrently and out of order: guard shared state yourself. A panic in a
handler is recovered and logged with its stack; the client keeps running. When
`Run` returns, the handlers' context is cancelled and `Run` waits for them first,
so no goroutine outlives it.

The bot's own messages are dispatched to it too; check `m.IsBot` (or compare
`m.UserID` with `Ready.User.ID`) to ignore them. `m.MentionsUser(id)` tells
whether a message mentions the bot.

`c.SetPresence(ctx, mnema.Presence{Status: "dnd", Activity: &mnema.Activity{Name: "chess", Type: mnema.ActivityPlaying}})`
sets the status (gateway op 3). It is kept and sent again with the next IDENTIFY
and after a resume; `Options.Presence` sets the starting one. The server applies
at most 5 updates a minute.

## REST

`c.REST` (or `mnema.NewREST(opts)` without a gateway) has one method per route:
`Me`, `Gateway`, `Channels`, `Messages(ctx, ch, mnema.MessageQuery{Limit, Before, After, Around})`,
`Message`, `CreateMessage(ctx, ch, mnema.MessageCreate{Content, Embeds, Components, AllowedMentions, MessageReference, ParentID, LifetimeSeconds})`,
`EditMessage(ctx, ch, id, mnema.MessageEdit{Content, Components})`, `DeleteMessage`,
`AddReaction`, `RemoveReaction`, `Upload(ctx, ch, mnema.Upload{Name, Reader, Content, ...})`,
`DownloadAttachment(ctx, id, mnema.DownloadOptions{MaxBytes, Timeout})`,
`Members`, `Member`, `Commands`, `SetCommands`, `CreateCommand`, `DeleteCommand`.
Ids are UUID strings. Optional fields of edits are pointers: `mnema.Ptr("text")`;
`Components: &[]mnema.Component{}` removes the components.

A non-2xx answer is a `*mnema.RESTError{Status, Code, Message}` with the bot
API's JSON error codes (`mnema.CodeUnknownChannel`, `CodeMissingPermissions`, ...):

```go
var re *mnema.RESTError
if errors.As(err, &re) && re.Code == mnema.CodeMissingPermissions { ... }
```

A `404` also matches `mnema.ErrNotFound`: `errors.Is(err, mnema.ErrNotFound)`.

### Downloading attachments

A message's `Attachments` carry `ID`, `URL` (`/api/bot/v1/media/{id}`),
`Filename`, `ContentType` and `Size`, next to the same values under Mnema's
internal names (`OriginalFilename`, `MimeType`, `SizeBytes`). A
`LifetimeSeconds` message carries `ExpiresAt`.
`DownloadAttachment` streams a file the bot can view:

```go
d, err := c.REST.DownloadAttachment(ctx, m.Attachments[0].ID, mnema.DownloadOptions{
	MaxBytes: 10 << 20, Timeout: 30 * time.Second,
})
if errors.Is(err, mnema.ErrNotFound) { ... } // gone, or not viewable by the bot
if err != nil { ... }
defer d.Body.Close()
data, err := io.ReadAll(d.Body) // a *mnema.MaxBytesError past MaxBytes
```

`d.ContentType`, `d.Filename` (from `Content-Disposition`) and `d.Size` (the
`Content-Length`, `-1` when unknown) describe the file. With `MaxBytes`, a larger
announced size is refused before the body is read, and a body that turns out
larger fails while reading, with a `*mnema.MaxBytesError`. `Timeout` covers
reading the body too; closing `Body` ends it. Redirects are not followed (a
`3xx` is a `*RESTError`). The server answers `404`, never `403`, for a file the
bot cannot view, so an id's existence is not leaked
([Downloading attachments](https://github.com/mnema-xyz/talk/blob/main/doc/bots.md#downloading-attachments)).

### Rate limits

The client keeps within the server's limits before it is told so: **50 requests
per second** per bot and **5 messages per 5 seconds per channel** for posts and
uploads (each channel has its own bucket, other routes have one per route). A
`429` is waited out (`retry_after` of the body, else the `Retry-After` header; a
`global` one holds back every request) and sent again, at most
`Options.MaxRetries` times (default 3); then the `429` is returned as a
`*RESTError` with `RetryAfter` and `Global`. Waiting stops when the context
ends. Slow mode answers `429` the same way. Requests carry
`User-Agent: mnema-talk-sdk-go/<module version>` and nothing personal.

## Slash commands and interactions

```go
_, err := c.REST.SetCommands(ctx, []mnema.CommandInput{{
	Name: "roll", Description: "Rolls a die",
	Options: []mnema.CommandOption{{Name: "sides", Description: "Sides", Type: mnema.OptionInteger}},
}})

c.OnInteraction(func(ctx context.Context, i *mnema.Interaction) {
	switch {
	case i.CommandName() == "roll":
		_ = i.DeferReply(ctx, false)
		_, _ = i.EditReply(ctx, mnema.Edit{Content: mnema.Ptr(fmt.Sprint(rand.Int64N(max(i.Option("sides").Int(), 2)) + 1))})
	case i.IsButton() && i.CustomID() == "again":
		_ = i.Update(ctx, mnema.Edit{Content: mnema.Ptr("again!")})
	}
})
```

An `Interaction` has `ID`, `Token`, `Type`, `ChannelID`, `User`, `Member`
(including channel permissions in `Member.Permissions`), `Message` (the
message a component is on) and `Data`, plus helpers: `CommandName()`,
`Option(name)` with `String()`, `Int()`, `Bool()` (zero values when missing),
`CustomID()`, `Values()`, `Field(customID)` for modal submits, `HasPermission(perm)`
to check member permissions in the channel, and `IsCommand`,
`IsComponent`, `IsButton`, `IsSelect`, `IsModalSubmit`.

### Command permissions (DefaultMemberPermissions)

A command can restrict who can see and run it in a channel with `DefaultMemberPermissions`:
- `nil`: available to everyone who can use commands in the channel.
- `"0"` or `[]string{}`: available to nobody except the Owner and members with Administrator.
- `mnema.Permission` bitfield (e.g. `mnema.PermissionBanMembers` or `mnema.PermissionAdministrator | mnema.PermissionManageChannels`), a slice of permission keys (e.g. `[]string{"ban_members"}`), or a decimal bitfield string (`"524288"`).

> [!WARNING]
> Mnema's permission bits have their own values: bit constants from other chat libraries do not match them. Use the `mnema.Permission` constants, pass permission keys (e.g. `[]string{"administrator"}`), or compute the bits from Mnema's catalogue (`GET /api/permissions`).

| Method | Callback | For |
|---|---|---|
| `Reply(ctx, mnema.Response{Content, Embeds, Components, AllowedMentions, Ephemeral})` | 4 | commands, components, modal submits |
| `DeferReply(ctx, ephemeral)` | 5 | the same; answer later with `EditReply` or `FollowUp` |
| `Update(ctx, mnema.Edit{...})` | 7 | components (and modal submits opened from one) |
| `DeferUpdate(ctx)` | 6 | the same |
| `ShowModal(ctx, mnema.Modal{...})` | 9 | commands and components |
| `EditReply(ctx, mnema.Edit{...})`, `DeleteReply(ctx)` | `PATCH`/`DELETE .../messages/@original` | after the first answer |
| `FollowUp(ctx, mnema.Response{...})` | `POST .../followups` | up to 5 responses in total |

The first answer must come within 15 minutes and can be given once (a second
one returns `mnema.ErrAlreadyAnswered`). `EditReply` and `FollowUp` return a
nil message for an ephemeral response, which is never stored. `Ephemeral`
belongs to `Reply`, `DeferReply` and `FollowUp`; `mnema.Edit` (for
`EditReply` and `Update`) has none, because the first response decides and
the server refuses flags on an edit. `AllowedMentions` works as on
`CreateMessage`: without it an answer notifies the members it names, never
`@all`, `@here` or a role. `i.User` is always set: the server sends the member
with every interaction. `Ready.User.Handle` is the name members mention the
bot by (for example `@mnemo`); `Ready.User.Username` is its display name.

## Components and modals

```go
rows := []mnema.Component{
	mnema.ActionRow(
		mnema.SuccessButton("deploy:v2.4", "Deploy"),
		mnema.SecondaryButton("cancel", "Cancel"),
		mnema.LinkButton("https://example.org/changes", "Changelog"),
	),
	mnema.ActionRow(mnema.StringSelect{CustomID: "env", Placeholder: "Environment", Options: []mnema.SelectOption{
		{Label: "Staging", Value: "staging"}, {Label: "Production", Value: "prod", Default: true},
	}}),
}
_, err := c.REST.CreateMessage(ctx, ch, mnema.MessageCreate{Content: "Deploy v2.4?", Components: rows})

modal := mnema.NewModal("feedback", "Feedback",
	mnema.TextInput{CustomID: "subject", Label: "Subject", MaxLength: mnema.Ptr(80)},
	mnema.TextInput{CustomID: "body", Label: "What happened?", Style: mnema.TextInputParagraph, Required: mnema.Ptr(false)},
)
```

Builders: `ActionRow`, `Button{...}` and `PrimaryButton`, `SecondaryButton`,
`SuccessButton`, `DangerButton`, `LinkButton`, `StringSelect{...}`,
`UserSelect{...}`, `RoleSelect{...}`, `ChannelSelect{...}`, `TextInput{...}`,
`NewModal`. `mnema.Component` is the plain JSON of the bot API and can be built by hand too.
`Validate()` on each builder, `mnema.ValidateComponents(rows)` and
`Modal.Validate()` check the documented limits (5 rows, 25 components, 5
buttons or one select per row, `custom_id` 1 to 100 characters and unique,
labels, options, placeholders, min/max values, text input lengths); the SDK
runs them before every request that carries components or a modal, and the
errors wrap `mnema.ErrInvalidComponent`. The full rules are in
[Message components](https://github.com/mnema-xyz/talk/blob/main/doc/bots.md#message-components) and
[Modals](https://github.com/mnema-xyz/talk/blob/main/doc/bots.md#modals).

## Reconnects

`Run(ctx)` fetches the gateway address (`GET /api/bot/v1/gateway`, or
`Options.GatewayURL`), connects with `Authorization: Bot <token>` on the
upgrade, and identifies with the intents and the presence. It sends
heartbeats every interval, the first after a random part of it; a heartbeat
whose ACK has not come by the next one drops the connection. After a drop it
reconnects and **resumes** (session ID and last sequence number, within the
server's 2 minutes) at READY's `resume_gateway_url`, of which only the path
and query are used: the scheme, host and port stay those of the address the
session connected to; when the server answers `INVALID_SESSION` it identifies
again on the same connection, and after `4000`, `4003` or `4005` it starts a
new session. Reconnects wait with exponential backoff and jitter from 1 to 60
seconds (at least 30 seconds after `4008`). Outgoing frames stay within 4096
bytes (`mnema.ErrFrameTooLarge`) and 120 a minute.

`Run` returns `nil` when the context ends, a `*mnema.CloseError` on the fatal
close codes `4004` (bad or revoked token, disabled or banned bot), `4013` and
`4014` (intents), and a `*mnema.RESTError` when `GET /gateway` answers 401 or
403.

## The add link

A bot's README can offer an add link that opens the server's confirm screen
with its manifest ([Adding a bot from a manifest](https://github.com/mnema-xyz/talk/blob/main/doc/bots.md#adding-a-bot-from-a-manifest)):

```go
link, err := mnema.AddBotLink("https://talk.example.org", mnema.Manifest{
	Name:        "Mnemo",
	Description: "Answers questions and keeps notes.",
	Intents:     (mnema.IntentGuilds | mnema.IntentGuildMessages).Names(),
	Permissions: []string{"view_channel", "send_messages", "add_reactions"},
	Commands:    commands,
})
// https://talk.example.org/admin/bots/add#manifest=<base64url JSON>
```

The manifest is validated first (name, description, https homepage, avatar
data URL, intent and permission names, commands) and travels in the URL
fragment, which the browser never sends to the server.

## Versioning and support

- [Semantic versioning](https://semver.org): minor versions add features, major
  versions may break backwards compatibility, and pre-1.0 versions (`v0.x.y`) may
  adjust the API while it settles. Every change is tracked in
  [CHANGELOG.md](CHANGELOG.md).
- Fixes, including security fixes, go into the latest version only.
- Report vulnerabilities privately according to [SECURITY.md](SECURITY.md).
- Questions and bugs: [issues](https://github.com/mnema-xyz/bot-sdk-go/issues).
  Contributions: [CONTRIBUTING.md](CONTRIBUTING.md).

## Development

```sh
go test -race ./...
```

The unit tests run offline against `httptest` servers, including a WebSocket
gateway.

### Contract tests

The **contract tests** (build tag `contract`) run against a real server and are
skipped without `MNEMA_CONTRACT_URL`:

| Variable | Description |
|---|---|
| `MNEMA_CONTRACT_URL` | The server URL, e.g. `http://127.0.0.1:8080` |
| `MNEMA_CONTRACT_TOKEN` | A bot token; the bot can view and send in the channel, allowed `GUILDS`, `GUILD_MESSAGES`, `GUILD_MESSAGE_REACTIONS` |
| `MNEMA_CONTRACT_CHANNEL` | The test channel ID |
| `MNEMA_CONTRACT_MEMBER_COOKIE` | The `Cookie` header of a member who can view and send in the channel |

```sh
MNEMA_CONTRACT_URL=http://127.0.0.1:8080 MNEMA_CONTRACT_TOKEN=mnb_... \
MNEMA_CONTRACT_CHANNEL=<channel id> MNEMA_CONTRACT_MEMBER_COOKIE='<Cookie header of a member>' \
go test -count=1 -tags=contract -run Contract ./...
```

CI runs them on every change twice, against Mnema Talk's `main` and against its
latest release: each `contract` job checks out the server, puts this SDK where the server's `TestSDKContract`
expects it (`talk/sdk/go`), and runs `go test -tags=integration -count=1 -v -run '^TestSDKContract$/^go$' ./internal/server`.

## License

[MIT](LICENSE). Earlier copies in the mnema-talk repository were AGPL-3.0-only;
0.1.0 and later are MIT. Mnema Talk itself stays AGPL-3.0. Security reports go to
[SECURITY.md](SECURITY.md).
