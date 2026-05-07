// Package keycode maps W3C `KeyboardEvent.code` strings to macOS virtual
// keycodes (HIToolbox kVK_* constants), so the frontend can capture a hotkey
// and the Go side can register it with Carbon's RegisterEventHotKey.
package keycode

// KeyInfo is the (macOS keycode, human-readable label) pair for a key.
type KeyInfo struct {
	Code  uint16 `json:"code"`
	Label string `json:"label"`
}

// Map from W3C event.code -> mac virtual keycode + display label.
// Sources: HIToolbox/Events.h (kVK_*) and the W3C UI Events KeyboardEvent code list.
var byEventCode = map[string]KeyInfo{
	// Letters
	"KeyA": {0x00, "A"}, "KeyB": {0x0B, "B"}, "KeyC": {0x08, "C"},
	"KeyD": {0x02, "D"}, "KeyE": {0x0E, "E"}, "KeyF": {0x03, "F"},
	"KeyG": {0x05, "G"}, "KeyH": {0x04, "H"}, "KeyI": {0x22, "I"},
	"KeyJ": {0x26, "J"}, "KeyK": {0x28, "K"}, "KeyL": {0x25, "L"},
	"KeyM": {0x2E, "M"}, "KeyN": {0x2D, "N"}, "KeyO": {0x1F, "O"},
	"KeyP": {0x23, "P"}, "KeyQ": {0x0C, "Q"}, "KeyR": {0x0F, "R"},
	"KeyS": {0x01, "S"}, "KeyT": {0x11, "T"}, "KeyU": {0x20, "U"},
	"KeyV": {0x09, "V"}, "KeyW": {0x0D, "W"}, "KeyX": {0x07, "X"},
	"KeyY": {0x10, "Y"}, "KeyZ": {0x06, "Z"},

	// Digits (top row)
	"Digit0": {0x1D, "0"}, "Digit1": {0x12, "1"}, "Digit2": {0x13, "2"},
	"Digit3": {0x14, "3"}, "Digit4": {0x15, "4"}, "Digit5": {0x17, "5"},
	"Digit6": {0x16, "6"}, "Digit7": {0x1A, "7"}, "Digit8": {0x1C, "8"},
	"Digit9": {0x19, "9"},

	// Whitespace / control
	"Space":     {0x31, "Space"},
	"Enter":     {0x24, "Return"},
	"NumpadEnter": {0x4C, "Return"},
	"Tab":       {0x30, "Tab"},
	"Escape":    {0x35, "Esc"},
	"Backspace": {0x33, "Delete"},
	"Delete":    {0x75, "Fwd Delete"},

	// Arrows
	"ArrowUp":    {0x7E, "↑"},
	"ArrowDown":  {0x7D, "↓"},
	"ArrowLeft":  {0x7B, "←"},
	"ArrowRight": {0x7C, "→"},

	// Function keys
	"F1": {0x7A, "F1"}, "F2": {0x78, "F2"}, "F3": {0x63, "F3"},
	"F4": {0x76, "F4"}, "F5": {0x60, "F5"}, "F6": {0x61, "F6"},
	"F7": {0x62, "F7"}, "F8": {0x64, "F8"}, "F9": {0x65, "F9"},
	"F10": {0x6D, "F10"}, "F11": {0x67, "F11"}, "F12": {0x6F, "F12"},
	"F13": {0x69, "F13"}, "F14": {0x6B, "F14"}, "F15": {0x71, "F15"},

	// Punctuation (US ANSI layout positions)
	"Minus":        {0x1B, "-"},
	"Equal":        {0x18, "="},
	"BracketLeft":  {0x21, "["},
	"BracketRight": {0x1E, "]"},
	"Backslash":    {0x2A, "\\"},
	"Semicolon":    {0x29, ";"},
	"Quote":        {0x27, "'"},
	"Comma":        {0x2B, ","},
	"Period":       {0x2F, "."},
	"Slash":        {0x2C, "/"},
	"Backquote":    {0x32, "`"},

	// ISO key (§ on Finnish/German/Nordic Mac keyboards, just left of "1").
	"IntlBackslash": {0x0A, "§"},

	// Page navigation
	"Home":     {0x73, "Home"},
	"End":      {0x77, "End"},
	"PageUp":   {0x74, "PgUp"},
	"PageDown": {0x79, "PgDn"},
}

// FromEventCode resolves a W3C event.code to a KeyInfo. The bool is false for
// unsupported keys (e.g. modifiers themselves like "ShiftLeft").
func FromEventCode(code string) (KeyInfo, bool) {
	info, ok := byEventCode[code]
	return info, ok
}

// FromMacCode is the reverse lookup, used to render the saved hotkey when no
// label is stored alongside it. Returns the first matching entry.
func FromMacCode(macCode uint16) (KeyInfo, bool) {
	for _, info := range byEventCode {
		if info.Code == macCode {
			return info, true
		}
	}
	return KeyInfo{}, false
}
