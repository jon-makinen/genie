// Package launchatlogin toggles whether macOS launches Genie automatically at
// login. The macOS 13+ SMAppService API does the heavy lifting from inside the
// app bundle — no helper executable required.
package launchatlogin

// State is what the UI needs to render the toggle.
type State struct {
	Enabled           bool `json:"enabled"`
	Supported         bool `json:"supported"`
	RequiresApproval  bool `json:"requires_approval"`
}
