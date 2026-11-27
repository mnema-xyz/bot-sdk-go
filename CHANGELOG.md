# Changelog

All notable changes to `github.com/mnema-xyz/bot-sdk-go`. Versions follow
[semver](https://semver.org).

## Unreleased

- The heartbeat interval from HELLO is clamped to between 1 second and 5 minutes, so a server that says `1` cannot make the client beat in a tight loop and one that says a day cannot leave a dead connection undetected.

## 0.4.0 (2026-11-20)

- The gateway resumes at READY's `resume_gateway_url` only when it names the host and port of the address the session connected to (or is relative), and then uses only its path and query on that address's scheme. Any other resume address, an unparsable one, or one with a `..` path segment falls back to the address the session connected to.
- The default REST client no longer follows redirects: a redirect answer fails the call with a `*RESTError` carrying its status. A client passed as `Options.HTTPClient` keeps its own redirect policy.
- Errors of failed REST requests and downloads name interaction and webhook routes with the token replaced by `:token`, also in the wrapped `*url.Error`. Routes under `/webhooks/{id}/{token}` (none in the SDK yet) share one rate-limit bucket per route, as interaction routes do.

## 0.3.0 (2026-11-19)

- Added `REST.DownloadAttachment` (`GET /media/{id}`) with `DownloadOptions{MaxBytes, Timeout}`: a size over `MaxBytes` fails with `*MaxBytesError` before or while reading, redirects are not followed, and a `404` matches `ErrNotFound`.
- Added `ErrNotFound`: `errors.Is(err, ErrNotFound)` is true for every REST `404`.
- Added the documented `Filename`, `ContentType` and `Size` fields to `Attachment`.
- Docs describe the API's own JSON, codes and permission bits without comparisons to other products.

## 0.2.0 (2026-11-15)

- Added `default_member_permissions` to `CommandInput` with support for `Permission` bitmasks, integer values, and permission names.
- Added `HasPermission` on `Interaction` and `InteractionMember` to evaluate member permissions against `Permission` bit flags.
- Added typed `Permission` constants for all Mnema Talk permissions.
- Documentation updates and removed legacy phrases from README.

## 0.1.0 (2026-11-14)

The first tagged version; it is the Go SDK of Mnema Talk 3.1.x in its own
repository with its history, under the MIT license (earlier copies in the
mnema-talk repository were AGPL-3.0-only), and the module path is now
github.com/mnema-xyz/bot-sdk-go.
