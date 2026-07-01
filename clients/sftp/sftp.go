package sftp

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

func SSHConnect(host, user, password string) (*sftp.Client, error) {
	hostKeyCallback, err := defaultHostKeyCallback()
	if err != nil {
		return nil, err
	}

	return SSHConnectWithHostKeyCallback(host, user, password, hostKeyCallback)
}

func SSHConnectWithHostKeyCallback(host, user, password string, hostKeyCallback ssh.HostKeyCallback) (*sftp.Client, error) {
	config := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.Password(password),
		},
		HostKeyCallback: hostKeyCallback,
	}

	conn, err := ssh.Dial("tcp", host, config)
	if err != nil {
		return nil, err
	}

	client, err := sftp.NewClient(conn)
	if err != nil {
		return nil, err
	}

	return client, nil
}

func defaultHostKeyCallback() (ssh.HostKeyCallback, error) {
	knownHostsFiles := []string{"/etc/ssh/ssh_known_hosts"}
	if homeDir, err := os.UserHomeDir(); err == nil {
		knownHostsFiles = append(knownHostsFiles, filepath.Join(homeDir, ".ssh", "known_hosts"))
	}

	existingFiles := make([]string, 0, len(knownHostsFiles))
	for _, file := range knownHostsFiles {
		if _, err := os.Stat(file); err == nil {
			existingFiles = append(existingFiles, file)
		} else if !os.IsNotExist(err) {
			return nil, fmt.Errorf("failed to stat known hosts file %s: %w", file, err)
		}
	}

	if len(existingFiles) == 0 {
		return nil, fmt.Errorf("no SSH known_hosts files found; add the SFTP host key to ~/.ssh/known_hosts or /etc/ssh/ssh_known_hosts")
	}

	callback, err := knownhosts.New(existingFiles...)
	if err != nil {
		return nil, fmt.Errorf("failed to load SSH known_hosts: %w", err)
	}

	return callback, nil
}
