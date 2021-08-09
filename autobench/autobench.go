package main

import (
	"io/ioutil"
	"log"
	"os"
	"fmt"
	"bytes"
	"path/filepath"

	"golang.org/x/crypto/ssh"
	kh "golang.org/x/crypto/ssh/knownhosts"
	"github.com/jessevdk/go-flags"
)

type Options struct {
	Verbose []bool `short:"v" long:"verbose" description:"Show verbose debug information"`
}

var opts Options
var parser = flags.NewParser(&opts, flags.Default)

func argparse() {
	fmt.Printf("parsing arguments\n")
	if _, err := parser.Parse(); err != nil {
		switch flagsErr := err.(type) {
		case flags.ErrorType:
			if flagsErr == flags.ErrHelp {
				os.Exit(0)
			}
			os.Exit(1)
		default:
			os.Exit(1)
		}
	}

	fmt.Printf("Verbosity: %v\n", opts.Verbose)
}


func main() {
	// var hostKey ssh.PublicKey
	// A public key may be used to authenticate against the remote
	// server by using an unencrypted PEM-encoded private key file.
	//
	// If you have an encrypted private key, the crypto/x509 package
	// can be used to decrypt it.

	argparse()

	return
	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(dir)

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
		log.Fatal("could not create hostkeycallback function: ", err)
	}

	config := &ssh.ClientConfig{
		User: "yuri",
		Auth: []ssh.AuthMethod{
			// Use the PublicKeys method for remote authentication.
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: hostKeyCallback,
	}

	// Connect to the remote server and perform the SSH handshake.
	client, err := ssh.Dial("tcp", "147.75.80.25:22", config)
	if err != nil {
		log.Fatalf("unable to connect: %v", err)
	}
	defer client.Close()
	
	ss, err := client.NewSession()
	if err != nil {
		log.Fatal("unable to create SSH session: ", err)
	}
	defer ss.Close()

	var stdoutBuf bytes.Buffer
	ss.Stdout = &stdoutBuf
	ss.Run("uname -a")
	log.Printf("--output:\n%s\n", stdoutBuf)
}
