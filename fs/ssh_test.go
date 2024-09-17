package fs

import (
	"testing"
)

func TestSSHFS(t *testing.T) {
	username := "foo"
	password := "pass"

	fs, err := NewSSHFS("localhost:2222", username, password)
	if err != nil {
		t.Fatalf("%v", err)
	}

	if err := populateFS(fs); err != nil {
		t.Fatalf("%v", err)
	}

	{
		files, err := fs.ReadDir("*.json")
		if err != nil {
			t.Fatalf("%v", err)
		}

		if len(files) != 2 {
			t.Errorf("expected 2 files, got %d", len(files))
		}
	}

	{
		files, err := fs.ReadDir("*.yaml")
		if err != nil {
			t.Fatalf("%v", err)
		}

		if len(files) != 1 {
			t.Errorf("expected 1 files, got %d", len(files))
		}
	}
}
