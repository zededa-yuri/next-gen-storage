package scpwork

import (
	"fmt"
	"strings"
	"github.com/tmc/scp"
	"golang.org/x/crypto/ssh"
)

// SendCommandSSH try to access SSH with timer and sends command
func SendCommandSSH(ip *string, port *int, user, password, command string, foreground bool) int {
	if *ip != "" {
		configSSH := &ssh.ClientConfig{
			User: user,
			Auth: []ssh.AuthMethod{
				ssh.Password(password),
			},
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
			Timeout:         defaults.DefaultRepeatTimeout,
		}
		conn, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", *ip, *port), configSSH)
		if err != nil {
			return err
		}
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
		for _, clb := range callbacks {
			clb()
		}
	}
	return nil
}

// SendFileSCP send a file over SCP
func SendFileSCP(ip *string, port *int, user, password, filename, destpath string) int {
	if ip != nil && *ip != "" {
		configSSH := &ssh.ClientConfig{
			User: user,
			Auth: []ssh.AuthMethod{
				ssh.Password(password),
			},
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
			Timeout:         defaults.DefaultRepeatTimeout,
		}

		conn, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", *ip, *port), configSSH)
		if err != nil {
			fmt.Printf("No ssh connections: %v", err)
			return err
		}

		session, err := conn.NewSession()
		if err != nil {
			fmt.Printf("Create new session failed: %v\n", err)
			return err
		}

		err = scp.CopyPath(filename, destpath, session)
		if err != nil {
			fmt.Printf("Copy file on guest VM failed: %v\n", err)
			return err
		}
	}
	return nil
}

// GetFileSCP get a file over SCP
func GetFileSCP(ip *string, port *int, user, password, filename, destpath string) int {
	return nil
}
