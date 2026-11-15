package mnema

import "strconv"

// Permission is a permission bit of Mnema Talk's permission catalogue (GET /api/permissions).
//
// WARNING: Mnema's permission bit values are NOT Discord's bit values!
// Do not reuse Discord's permission constants or PermissionFlagsBits.
// Compute bit values from this catalogue or pass permission keys.
type Permission uint64

const (
	// PermissionAdministrator grants every permission below (Owner powers stay
	// with the Owner) and ignores channel overwrites.
	PermissionAdministrator Permission = 1 << iota
	// PermissionViewChannel lets a member see a channel (or Talk) and read its history.
	PermissionViewChannel
	PermissionSendMessages
	PermissionAttachFiles
	PermissionAddReactions
	PermissionManageMessages
	PermissionMentionEveryone
	PermissionConnect
	PermissionSpeak
	PermissionVideo
	PermissionDisconnectMembers
	PermissionManageChannels
	PermissionManageRoles
	PermissionCreateInvites
	PermissionManageWebhooks
	PermissionManageMembers
	PermissionManageBots
	PermissionViewAuditLog          Permission = 1 << 17
	PermissionTimeoutMembers        Permission = 1 << 18
	PermissionBanMembers            Permission = 1 << 19
	PermissionManageEmoji           Permission = 1 << 20
	PermissionManageReports         Permission = 1 << 21
	PermissionSendDirectMessages    Permission = 1 << 22
	PermissionKickMembers           Permission = 1 << 23
	PermissionManageStage           Permission = 1 << 24
	PermissionManageEvents          Permission = 1 << 25
	PermissionBypassNewMemberLimits Permission = 1 << 26
	PermissionEditWiki              Permission = 1 << 27
	PermissionManageWiki            Permission = 1 << 28
	PermissionViewWiki              Permission = 1 << 29
	PermissionCreateMemberInvites   Permission = 1 << 30
)

// Has reports whether the permission set contains the given permission bit.
func (p Permission) Has(other Permission) bool {
	return p&PermissionAdministrator != 0 || p&other == other
}

// String returns the decimal bitfield string of the permission.
func (p Permission) String() string {
	return strconv.FormatUint(uint64(p), 10)
}

// FormatPermissions returns a decimal string bitfield combining the given permissions.
func FormatPermissions(perms ...Permission) string {
	var combined Permission
	for _, p := range perms {
		combined |= p
	}
	return strconv.FormatUint(uint64(combined), 10)
}

// HasPermission reports whether a decimal permission bitfield string includes the permission.
// Administrator grants all permissions.
func HasPermission(rawPermissions string, p Permission) bool {
	if rawPermissions == "" {
		return false
	}
	bits, err := strconv.ParseUint(rawPermissions, 10, 64)
	if err != nil {
		return false
	}
	set := Permission(bits)
	return set&PermissionAdministrator != 0 || set&p == p
}
