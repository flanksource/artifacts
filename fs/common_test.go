package fs

import (
	"context"
	"strings"
)

func populateFS(fs FilesystemRW) error {
	testData := []struct {
		name    string
		content string
	}{
		{"first.json", "first"},
		{"second.json", "second"},
		{"third.yaml", "third"},
	}

	for _, td := range testData {
		_, err := fs.Write(context.TODO(), td.name, strings.NewReader(td.content))
		if err != nil {
			return err
		}
	}

	return nil
}
