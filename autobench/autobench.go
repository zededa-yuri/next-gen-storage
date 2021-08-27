package main

import (
	"io/ioutil"
	"log"
	"os"
	"fmt"
	"github.com/zededa-yuri/nextgen-storage/autobench/pkg/sshwork"
	"github.com/zededa-yuri/nextgen-storage/autobench/pkg/mkconfig"
	"github.com/zededa-yuri/nextgen-storage/autobench/pkg/fioconv"
	//	"bytes"
	// "path/filepath"

	"golang.org/x/crypto/ssh"
	kh "golang.org/x/crypto/ssh/knownhosts"
	// "github.com/pkg/sftp"
	//	"io"
	//	"bufio"

)


func run(ip, user string) {

	//*
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
		User: user,
		Auth: []ssh.AuthMethod{
			// Use the PublicKeys method for remote authentication.
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: hostKeyCallback,
	}

	// Connect to the remote server and perform the SSH handshake.
	client, err := ssh.Dial("tcp", ip, config)
	if err != nil {
		log.Fatalf("unable to connect: %v", err)
	}
	defer client.Close()

	mkconfig.GenerateFIOConfig(
		mkconfig.OpType{"read", "write"},
		mkconfig.BlockSize{"4k", "64k", "1m"},
		mkconfig.JobsType{1, 8},
		mkconfig.DepthType{1, 8, 32},
		"60",
		"fio_config.cfg",
	)

	if err := sshwork.SendFileSCP(
		client,
		"/Users/vk_en/Documents/src/next-gen-storage/autobench/fio_config.cfg",
		"/users/vit/fio_config.cfg",
	); err != nil {
		fmt.Println(err)
	}

	if err := sshwork.SendCommandSSH(
		client,
		"mkdir /users/vit/HELLO",
		true,
	); err != nil {
		fmt.Println(err)
	}


	if err := sshwork.GetFileSCP(
		client,
		"/Users/vk_en/Documents/src/next-gen-storage/autobench/fio_config.cfg-test1",
		"/users/vit/fio_config.cfg",
	); err != nil {
		fmt.Println(err)
	}

	fioconv.ConvertJSONtoCSV(
		"/Users/vk_en/Documents/src/next-gen-storage/autobench/result.json",
		"/Users/vk_en/Documents/src/next-gen-storage/autobench/fio_res.csv",
	)
}


func main() {
	run("145.40.93.205:22", "vit")
}
