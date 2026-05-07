// Package permissions inspects and triggers macOS Privacy permission prompts.
package permissions

// Status reports the booleans the settings UI shows.
type Status struct {
	Microphone    bool `json:"microphone"`
	Accessibility bool `json:"accessibility"`
}
