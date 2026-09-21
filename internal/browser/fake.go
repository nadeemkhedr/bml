package browser

// Call records a single invocation of Fake.OpenOrFocus.
type Call struct {
	URL      string
	ForceNew bool
}

// Fake is a Browser test double that records calls instead of touching a real
// browser. It is the spine of bml's test suite — CLI and TUI tests inject it to
// assert which URL was acted on and whether a new tab was forced. It also
// implements TabLister and HistoryLister so tab- and browse-mode tests can drive
// the models with canned tabs and history.
type Fake struct {
	Calls []Call
	// Err, if set, is returned from OpenOrFocus to exercise error paths.
	Err error
	// Tabs is returned from ListTabs; TabsErr, if set, is returned instead to
	// exercise the not-running / automation-denied paths.
	Tabs    []Tab
	TabsErr error
	// Visits is returned from ListHistory; HistoryErr, if set, is returned
	// instead to exercise the fall-back-to-bookmarks path.
	Visits     []Visit
	HistoryErr error
}

// OpenOrFocus implements Browser.
func (f *Fake) OpenOrFocus(url string, forceNew bool) error {
	f.Calls = append(f.Calls, Call{URL: url, ForceNew: forceNew})
	return f.Err
}

// ListTabs implements TabLister.
func (f *Fake) ListTabs() ([]Tab, error) {
	return f.Tabs, f.TabsErr
}

// ListHistory implements HistoryLister.
func (f *Fake) ListHistory() ([]Visit, error) {
	return f.Visits, f.HistoryErr
}

// Last returns the most recent call and whether any call was recorded.
func (f *Fake) Last() (Call, bool) {
	if len(f.Calls) == 0 {
		return Call{}, false
	}
	return f.Calls[len(f.Calls)-1], true
}
