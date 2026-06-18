package cli

import (
	"os"
	"testing"

	"bml/internal/history"
)

func TestHistoryClear_RemovesFile(t *testing.T) {
	dir := tempConfig(t, sampleConfig)
	path := history.Path(dir)
	if err := os.WriteFile(path, []byte("[[entry]]\nquery=\"g\"\nurl=\"https://x\"\nrank=1.0\nlast=2026-06-18T12:00:00Z\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := run(t, "--config", dir, "history", "clear"); err != nil {
		t.Fatalf("history clear: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("history file should be gone, stat err = %v", err)
	}
}

func TestHistoryClear_NoFileIsNotAnError(t *testing.T) {
	dir := tempConfig(t, sampleConfig)
	if _, err := run(t, "--config", dir, "history", "clear"); err != nil {
		t.Errorf("clearing with no history should succeed, got %v", err)
	}
}
