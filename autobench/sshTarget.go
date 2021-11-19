package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"time"

	"github.com/zededa-yuri/nextgen-storage/autobench/pkg/fiotests"
	"golang.org/x/crypto/ssh"
	kh "golang.org/x/crypto/ssh/knownhosts"
)

type SSHCommand struct {
	SSHhost string `short:"a" long:"adress" description:"IP adress for ssh connections."`
	SSHPort int    `short:"p" long:"port" description:"Port for ssh connections." default:"22"`
	SSHUser string `short:"u" long:"user" description:"A user name for ssh connections"`
	SSHPass string `short:"s" long:"pass" description:"Password for ssh connections. If no set, using key"`
}

var sshCmd SSHCommand

func getConnectionPass(port int, ip, user, pass string) (*ssh.Client, error) {
	config := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.Password(pass),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
	// Connect to the remote server and perform the SSH handshake.
	client, err := ssh.Dial("tcp", ip, config)
	if err != nil {
		return nil, fmt.Errorf("unable to connect: %w", err)
	}
	return client, nil
}

func getConnectionKey(port int, ip, user string) (*ssh.Client, error) {
	home := os.Getenv("HOME")
	key_path := fmt.Sprintf("%s/.ssh/id_rsa", home)
	log.Printf("Loading keyfile %s\n", key_path)
	key, err := ioutil.ReadFile(key_path)
	if err != nil {
		log.Fatalf("unable to read private key: %v", err)
	}

	// Create the Signer for this private key.
	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		log.Fatalf("unable to parse private key: %v", err)
	}

	known_hosts_path := fmt.Sprintf("%s/.ssh/known_hosts", home)
	hostKeyCallback, err := kh.New(known_hosts_path)
	if err != nil {
		log.Fatalf("could not create hostkeycallback function: %v", err)
	}

	config := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: hostKeyCallback,
	}

	// Connect to the remote server and perform the SSH handshake.
	client, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", ip, port), config)
	if err != nil {
		return nil, fmt.Errorf("unable to connect: %w", err)
	}
	return client, nil
}

func (x *SSHCommand) Execute(args []string) error {
	var sshClient *ssh.Client
	err := InitFioOptions()
	if err != nil {
		return fmt.Errorf("error get fio params: %w", err)
	}

	if sshCmd.SSHPass != "" {
		sshClient, err  = getConnectionPass(sshCmd.SSHPort, sshCmd.SSHhost, sshCmd.SSHUser, sshCmd.SSHPass)
		if err != nil {
			return fmt.Errorf("get ssh client (via password) failed: %v", err)
		}
	} else {
		sshClient, err  = getConnectionKey(sshCmd.SSHPort, sshCmd.SSHhost, sshCmd.SSHUser)
		if err != nil {
			return fmt.Errorf("get ssh client (via key) failed: %v", err)
		}
	}
	defer sshClient.Close()

	fmt.Println("FIO Tests start...")
	err = fiotests.RunFIOTest(sshClient, sshCmd.SSHUser, opts.LocalFolderResults, opts.LocalDirResults, opts.TargetFIODevice, FioOptions, time.Duration(opts.TimeOneTest) * time.Second);
	if err != nil {
		return fmt.Errorf("FIO tests failed error: %v", err)
	}

	fmt.Println("FIO Tests finished!")
	return nil
}

func init() {
	parser.AddCommand(
		"ssh",
		"Create test and run on remote machine",
		"This command starts Test on remote machine",
		&sshCmd,
	)
}
