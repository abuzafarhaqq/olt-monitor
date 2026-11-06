package auth

// Permission ids are the same strings the frontend uses in
// src/services/auth.ts. The browser hides controls; this table decides what the
// API actually allows, and the two must not drift.
type Permission string

const (
	PermDeviceRead      Permission = "device.read"
	PermDeviceManage    Permission = "device.manage"
	PermDiagnosticsRead Permission = "diagnostics.read"
	PermDiagnosticsRun  Permission = "diagnostics.run"
	PermIncidentRead    Permission = "incident.read"
	PermIncidentAck     Permission = "incident.acknowledge"
	PermIncidentNote    Permission = "incident.note"
	PermIncidentResolve Permission = "incident.resolve"
	PermReportsRead     Permission = "reports.read"
	PermReportsExport   Permission = "reports.export"
	PermSettingsRead    Permission = "settings.read"
	PermSettingsManage  Permission = "settings.manage"
	PermUsersManage     Permission = "users.manage"
)

var readOnly = []Permission{PermDeviceRead, PermDiagnosticsRead, PermIncidentRead, PermReportsRead}

// RolePermissions mirrors ROLE_PERMISSIONS in the frontend. A test asserts the
// two lists agree on the permissions that exist.
var RolePermissions = map[string][]Permission{
	"viewer": readOnly,
	"noc-operator": concat(readOnly,
		PermIncidentAck, PermIncidentNote, PermReportsExport),
	"network-engineer": concat(readOnly,
		PermIncidentAck, PermIncidentNote, PermIncidentResolve, PermReportsExport,
		PermDiagnosticsRun, PermDeviceManage, PermSettingsRead),
	"administrator": concat(readOnly,
		PermIncidentAck, PermIncidentNote, PermIncidentResolve, PermReportsExport,
		PermDiagnosticsRun, PermDeviceManage, PermSettingsRead, PermSettingsManage, PermUsersManage),
}

func concat(base []Permission, extra ...Permission) []Permission {
	out := make([]Permission, 0, len(base)+len(extra))
	out = append(out, base...)
	return append(out, extra...)
}

// Can answers the question the HTTP layer actually asks.
func Can(role string, permission Permission) bool {
	for _, granted := range RolePermissions[role] {
		if granted == permission {
			return true
		}
	}
	return false
}
