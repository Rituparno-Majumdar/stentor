// Package content defines the platform-agnostic post payload every
// platforms.Platform implementation accepts, and the result it returns.
package content

// Content is what the user wants posted, independent of platform.
type Content struct {
	Text  string // required by nearly every platform
	Image string // optional local file path
	Link  string // optional URL to attach/append
}

// Result is what a successful post returns, for the CLI to print back.
type Result struct {
	Platform string
	URL      string // permalink, if the platform's API returns one
	ID       string // platform-native post/record id
}
