package fs

// TODO: Setup SMB server on CI
// func TestSMB(t *testing.T) {
// 	password := os.Getenv("NAS_SMB_PASSWORD")

// 	fs, err := NewSMBFS("localhost", "445", "aditya", types.Authentication{
// 		Username: types.EnvVar{ValueStatic: "aditya"},
// 		Password: types.EnvVar{ValueStatic: password}})
// 	if err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	files, err := fs.ReadDir("documents/**/*.pdf")
// 	if err != nil {
// 		t.Fatalf(err.Error())
// 	}

// 	for _, p := range files {
// 		fmt.Printf("file: %s\n", p.FullPath())
// 	}
// }
