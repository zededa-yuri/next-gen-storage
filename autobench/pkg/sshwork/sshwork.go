package sshwork

import (
	"fmt"
	"os"
	"io/ioutil"
	"golang.org/x/crypto/ssh"
	//kh "golang.org/x/crypto/ssh/knownhosts"
	"github.com/pkg/sftp"
)

// SendCommandSSH try to access SSH with timer and sends command
func SendCommandSSH(conn *ssh.Client, command string, foreground bool) error {
	session, _ := conn.NewSession()
	if foreground {
		defer session.Close()
		if err := session.Run(command); err != nil {
			fmt.Println(err)
			return err
		}
	} else {
		go func() {
			_ = session.Run(command) //we cannot get answer for this command
			session.Close()
		}()
	}

	return nil
}

// SendFileSCP send a file over SCP
func SendFileSCP(conn *ssh.Client, localPath, remotePath string) error {
	client, err := sftp.NewClient(conn)
	if err != nil {
		return err
	}
	defer client.Close()

	fmt.Printf("using open\n")
	rem_f, err := client.OpenFile(remotePath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY)
	if err != nil {
		return err
	}
	defer rem_f.Close()

	loc_f, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer loc_f.Close()

	buf, err := ioutil.ReadAll(loc_f)
	if _, err := rem_f.Write(buf); err != nil {
		return err
	}
	return nil
}

// GetFileSCP get a file over SCP
func GetFileSCP(conn *ssh.Client, localPath, remotePath string) error {
	client, err := sftp.NewClient(conn)
	if err != nil {
		return err
	}
	defer client.Close()

	rem_f, err := client.Open(remotePath)
	if err != nil {
		return fmt.Errorf("Cannot open remote file [%s]: %w", remotePath, err)
	}
	defer rem_f.Close()

	loc_f, err := os.OpenFile(localPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0755)
	if err != nil {
		return fmt.Errorf("Cannot open local file: %w", err)
	}
	defer loc_f.Close()

	buf, err := ioutil.ReadAll(rem_f)
	if _, err := loc_f.Write(buf); err != nil {
		return err
	}
	return nil
}
