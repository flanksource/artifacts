package fs

import "testing"

func TestLocalFSGlob(t *testing.T) {
	testData := []struct {
		pattern string
		want    int
	}{
		{
			pattern: "obj-*.json",
			want:    2,
		},
		{
			pattern: "*.json",
			want:    3,
		},
		{
			pattern: "*.txt",
			want:    0,
		},
	}

	fs := NewLocalFS("testdata")
	for _, td := range testData {
		t.Run(td.pattern, func(t *testing.T) {
			list, err := fs.ReadDir(td.pattern)
			if err != nil {
				t.Fatal(err)
			}

			if len(list) != td.want {
				t.Fatalf("expected %d files, got %d", td.want, len(list))
			}
		})
	}
}
