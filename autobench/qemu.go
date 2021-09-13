package main

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"text/template"
	"time"

	"github.com/zededa-yuri/nextgen-storage/autobench/qemutmp"
	"golang.org/x/crypto/ssh"
	kh "golang.org/x/crypto/ssh/knownhosts"
)


type QemuCommand struct {
	CQemuConfigDir 	string `short:"c" long:"config" description:"The option takes the path to the QEMU configuration file"`
	CFileLocation 	string `short:"i" long:"image" description:"The option takes the path to the .img file" default:"bionic-server-cloudimg-i386.img"`
	CFormat 		string `short:"f" long:"format" description:"Format options " default:"raw"`
	CVCpus 			string `short:"v" long:"vcpu" description:"VCpu and core counts" default:"2"`
	CMemory			string `short:"m" long:"memory" description:"RAM memory value" default:"512"`
}

var qemu_command QemuCommand
type VmConfig struct {
	FileLocation 	string // default "bionic-server-cloudimg-i386.img"
	Format 			string // default "raw"
	VCpus 			string // default "2"
	Memory 			string // default "512"
	Kernel 			string // default ""
}

func write_main_config(path string, template_args VmConfig) error {
	t, err := template.New("qemu").Parse(qemutmp.QemuConfTemplate)
	if err != nil {
		fmt.Printf("failed parse template%v\n", err)
		return err
	}

	config_f, err := os.OpenFile(path,
		os.O_RDWR | os.O_CREATE | os.O_TRUNC,
		0644)

	if err != nil {
		fmt.Printf("failed to open file %s: %v\n", path, err)
		return err
	}
	defer config_f.Close()

	err = t.Execute(config_f, template_args)
	if err !=  nil {
		fmt.Printf("cant parse template")
		return err
	}

	return nil
}

type SshConnection struct {
	//client *ssh.Client
}

func (connection SshConnection) Init(ctx context.Context, ssHport int) error {
	home := os.Getenv("HOME")
	key_path := fmt.Sprintf("%s/.ssh/id_rsa", home)
	log.Printf("Loading keyfile %s\n----------------------------\n", key_path)
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
		User: "ubuntu",
		Auth: []ssh.AuthMethod{
			// Use the PublicKeys method for remote authentication.
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: hostKeyCallback,
	}


	time.Sleep(3 * time.Second) // For waiting create VM
	for i := 0; i < 30; i++ {
		_, err := ssh.Dial("tcp", fmt.Sprintf("localhost:%d", ssHport), config)
		if err != nil {
			log.Printf("Unable to connect: 127.0.0.1:%d err:%v", ssHport, err)
		} else {
			log.Printf("Connection to: 127.0.0.1:%d was successful", ssHport)
			break
		}

		if ctx.Err() == context.Canceled || ctx.Err() == context.DeadlineExceeded {
			return ctx.Err()
		}
		time.Sleep(3 * time.Second)
	}

	if err != nil {
		log.Printf("failed to establish connection after serveral attempts\n")
		return err
	}
	// Connect to the remote server and perform the SSH handshake.
	return nil
}

func get_self_path() (string) {
	ex, err := os.Executable()
	if err != nil {
		panic(err)
	}
	return filepath.Dir(ex)
}

func qemu_run(ctx context.Context, cancel context.CancelFunc, qemuConfigDir string, port int) {

	cmd := exec.CommandContext(ctx,
		"qemu-system-x86_64",
		"-readconfig",  qemuConfigDir,
		"-display", "none",
		"-device", "e1000,netdev=net0",  "-netdev",  fmt.Sprintf("user,id=net0,hostfwd=tcp::%d-:22", port),
		"-serial", "chardev:ch0")

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	err := cmd.Run() //no have end for this command
	if ctx.Err() == context.Canceled || ctx.Err() == context.DeadlineExceeded {
		fmt.Printf("Cancelled via timer: %v\n", ctx.Err())
		return
	} else if err != nil {
		fmt.Printf("error launching command: %v; err=%v\nStdout:\n%s\n", err, ctx.Err(), out)
		cancel()
	} else {
		cancel()
	}
}

type Target interface {
	init(ctx context.Context, cancel context.CancelFunc, timeWork time.Duration)
	RunCommand(command string, foreground bool) error
}

type TargetQemu struct {
	connection SshConnection
}

func (qemu *TargetQemu) init(ctx context.Context, cancel context.CancelFunc, timeWork time.Duration) {
	template_args := VmConfig{}
	template_args.FileLocation = qemu_command.CFileLocation
	template_args.Format = qemu_command.CFormat
	template_args.VCpus = qemu_command.CVCpus
	template_args.Memory = qemu_command.CMemory

	qemuConfigDir := filepath.Join(get_self_path(), "qemu.cfg")
	err := write_main_config(qemuConfigDir, template_args)
	if err != nil {
		return
	}


	for i := 0; i < opts.CCountVM; i++ {
		go qemu_run(ctx, cancel, qemuConfigDir, opts.CPort + i)
		log.Printf("Create VM by the address [localhost:%d] with limit time in %v\n", opts.CPort + i, timeWork)
	}
	fmt.Println("--------------------------------------------------------")

	// FIX ME FIRST COMMECTION FOR KNOWN HOSTS

	for i := 0; i < opts.CCountVM; i++ {
		err := qemu.connection.Init(ctx, opts.CPort + i)
		if err != nil {
			fmt.Println("connection to VM on address failed:", opts.CPort + i, err)
		} else {
			break
		}
	}

}

func (qemu *TargetQemu) RunCommand(command string, foreground bool) error {
	/* TODO */
	return nil
}

func (x *QemuCommand) Execute(args []string) error {
	ctx := context.Background()
	ctxVM, cancel := context.WithTimeout(ctx, 60 * time.Minute)
	defer cancel()

	qemu := TargetQemu{}
	qemu.init(ctxVM, cancel, 60 * time.Minute)

	//need waiting system

	return nil
}

func init() {
	parser.AddCommand("qemu",
		"Create and run VM in QEMU",
		"This command creates and starts a virtual machine in QEMU",
		&qemu_command)
}
