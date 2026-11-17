# Changelog

All notable changes to `github.com/mnema-xyz/bot-sdk-go`. Versions follow
[semver](https://semver.org).

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
