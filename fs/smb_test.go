package fs

import (
	"testing"

	"github.com/flanksource/duty/types"
)

func TestSMB(t *testing.T) {
	username := "example1"
	password := "badpass"
	share := "users"

	fs, err := NewSMBFS("localhost", "445", share, types.Authentication{
		Username: types.EnvVar{ValueStatic: username},
		Password: types.EnvVar{ValueStatic: password}})
	if err != nil {
		t.Fatalf("%v", err)
	}

	if err := populateSMB(fs); err != nil {
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

func populateSMB(fs *smbFS) error {
	testData := []struct {
		name    string
		content string
	}{
		{"first.json", "first"},
		{"second.json", "second"},
		{"third.yaml", "third"},
	}

	for _, td := range testData {
		f, err := fs.Create(td.name)
		if err != nil {
			return err
		}
		defer f.Close()

		if _, err := f.Write([]byte(td.content)); err != nil {
			return err
		}
	}

	return nil
}
