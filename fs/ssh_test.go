package fs

// TODO: Setup SFTP server on CI
// func TestSSHFS(t *testing.T) {
// 	password := os.Getenv("NAS_SMB_PASSWORD")

// 	fs, err := NewSSHFS("localhost:22", "admin", password)
// 	if err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	files, err := fs.ReadDir("/mnt/mega/aditya/documents/**/*.pdf")
// 	if err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	for _, p := range files {
// 		fmt.Printf("file: %s\n", p.FullPath())
// 	}
// }
