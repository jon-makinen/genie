// Package inject types text into the currently focused application. The
// platform-specific implementation lives in inject_<os>.go.
package inject

// Injector is the small interface the pipeline uses to write transcripts.
type Injector interface {
	Type(text string) error
}
