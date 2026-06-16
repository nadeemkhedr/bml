package browser

// Call records a single invocation of Fake.OpenOrFocus.
type Call struct {
	URL      string
	ForceNew bool
}

// Fake is a Browser test double that records calls instead of touching a real
// browser. It is the spine of bml's test suite — CLI and TUI tests inject it to
// assert which URL was acted on and whether a new tab was forced.
type Fake struct {
	Calls []Call
	// Err, if set, is returned from OpenOrFocus to exercise error paths.
	Err error
}

// OpenOrFocus implements Browser.
func (f *Fake) OpenOrFocus(url string, forceNew bool) error {
	f.Calls = append(f.Calls, Call{URL: url, ForceNew: forceNew})
	return f.Err
}

// Last returns the most recent call and whether any call was recorded.
func (f *Fake) Last() (Call, bool) {
	if len(f.Calls) == 0 {
		return Call{}, false
	}
	return f.Calls[len(f.Calls)-1], true
}
