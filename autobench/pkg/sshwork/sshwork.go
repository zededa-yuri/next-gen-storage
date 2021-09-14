package sshwork

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"

)

// SendCommandSSH sends command
func SendCommandSSH(sshClient *ssh.Client, command string, foreground bool) error {
	session, err := sshClient.NewSession()
	if err != nil {
		return fmt.Errorf("could not create new ssh session: %w", err)
	}
	defer session.Close()

	if foreground {
		if err := session.Run(command); err != nil {
			return fmt.Errorf("could not run command [%s]: %w", command, err)
		}
	} else {
		go func() {
			_ = session.Run(command) // we cannot get answer for this command
		}()
	}

	return nil
}

// SendFileSCP send a file over SCP
func SendFileSCP(sshClient *ssh.Client, localPath, remotePath string) error {
	client, err := sftp.NewClient(sshClient)
	if err != nil {
		return fmt.Errorf("could not create new sftp.Client: %w", err)
	}
	defer client.Close()

	remoteFile, err := client.OpenFile(remotePath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY)
	if err != nil {
		return fmt.Errorf("could not open remote file [%s] for writing: %w", remotePath, err)
	}
	defer remoteFile.Close()

	localFile, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("could not open local file [%s] for reading: %w", localPath, err)
	}
	defer localFile.Close()

	buf, err := ioutil.ReadAll(localFile)
	if err != nil {
		return fmt.Errorf("could not read from local file: %w", err)
	}

	if _, err := remoteFile.Write(buf); err != nil {
		return fmt.Errorf("could not write to remote file: %w", err)
	}

	return nil
}

// GetFileSCP get a file over SCP
func GetFileSCP(sshClient *ssh.Client, localPath, remotePath string) error {
	client, err := sftp.NewClient(sshClient)
	if err != nil {
		return fmt.Errorf("could not create new sfpt.Client: %w", err)
	}
	defer client.Close()

	remoteFile, err := client.Open(remotePath)
	if err != nil {
		return fmt.Errorf("could not open remote file [%s] for reading: %w", remotePath, err)
	}
	defer remoteFile.Close()

	localFile, err := os.OpenFile(localPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0755)
	if err != nil {
		return fmt.Errorf("could not open local file [%s] for writing: %w", localPath, err)
	}
	defer localFile.Close()

	buf, err := ioutil.ReadAll(remoteFile)
	if err != nil {
		return fmt.Errorf("could not read from remote file: %w", err)
	}

	if _, err := localFile.Write(buf); err != nil {
		return fmt.Errorf("could not write to local file: %w", err)
	}

	return nil
}
