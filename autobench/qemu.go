package main

import (
	"io/ioutil"
	"log"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"os/exec"
	"context"
	"time"
	"text/template"
	"golang.org/x/crypto/ssh"
	kh "golang.org/x/crypto/ssh/knownhosts"
	"github.com/zededa-yuri/nextgen-storage/autobench/qemutmp"
)


type QemuCommand struct {
	//Verbose []bool `short:"v" long:"verbose" description:"Show verbose debug information"`
	CQemuConfigDir 	string `short:"c" long:"config" description:"The option takes the path to the QEMU configuration file"`
	CFileLocation 	string `short:"i" long:"image" description:"The option takes the path to the .img file" default:"bionic-server-cloudimg-i386.img"`
	CFormat 		string `short:"f" long:"format" description:"Format options " default:"raw"`
	CVCpus 			string `short:"v" long:"vcpu" description:"VCpu and core counts" default:"2"`
	CMemory			string `short:"m" long:"memory" description:"RAM memory value" default:"512"`
	//CPort			int	`short:"p" long:"port" description:"Port for connect to VM" default:"6666"`
	//CCountVM		int `short:"n" long:"number" description:"Count create VM" default:"1"`
	// CKernel		 	string `short:"k" long:"kernel" description:"[Options] Path to the kernel"`
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
	client *ssh.Client
}

func (connection SshConnection) Init(ctx context.Context, ssHport int) error {
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
		User: "ubuntu",
		Auth: []ssh.AuthMethod{
			// Use the PublicKeys method for remote authentication.
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: hostKeyCallback,
	}

	var client *ssh.Client
	for i := 0; i < 30; i++ {
		log.Printf("Dialing in\n")
		client, err = ssh.Dial("tcp", fmt.Sprintf("localhost:%d", ssHport), config)
		if err != nil {
			log.Printf("unable to connect: %v", err)
		} else {
			break
		}
		log.Printf("Sleeping 5 sec\n")
		time.Sleep(5 * time.Second)
		if ctx.Err() == context.Canceled || ctx.Err() == context.DeadlineExceeded {
			return ctx.Err()
		}
	}

	if err != nil {
		log.Printf("failed to establish connection after serveral attempts\n")
		return err
	}
	log.Printf("Connection for address [%s] success\n", fmt.Sprintf("localhost:%d", ssHport))
	// Connect to the remote server and perform the SSH handshake.
	connection.client = client
	return nil
}

func get_self_path() (string) {
	ex, err := os.Executable()
	if err != nil {
		panic(err)
	}
	return filepath.Dir(ex)
}

func qemu_run(ctx context.Context, cancel context.CancelFunc) {

	//Init args for template
	template_args := VmConfig{}
	template_args.FileLocation = qemu_command.CFileLocation
	template_args.Format = qemu_command.CFormat
	template_args.VCpus = qemu_command.CVCpus
	template_args.Memory = qemu_command.CMemory

	qemuConfigDir := filepath.Join(get_self_path(), "configs/qemu.cfg")
	err := write_main_config(qemuConfigDir, template_args)
	if err != nil {
		return
	}

	if opts.CCountVM == 1 {}
	var cmd *exec.Cmd
	for i := 0; i < opts.CCountVM; i++ {
		cmd = exec.CommandContext(ctx,
			"qemu-system-x86_64",
			"-readconfig",  qemuConfigDir,
			"-display", "none",
			"-device", "e1000,netdev=net0",  "-netdev",  fmt.Sprintf("user,id=net0,hostfwd=tcp::%d-:22", opts.CPort + i),
			"-serial", "chardev:ch0")

		var out bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &out

		fmt.Printf("Create VM \n")
		err = cmd.Run()

		if ctx.Err() == context.Canceled || ctx.Err() == context.DeadlineExceeded {
			fmt.Printf("Cancelled: %v\n", ctx.Err())
			return
		} else if err != nil {
			fmt.Printf("error launching command: %v; err=%v\n", err, ctx.Err())
			fmt.Printf("%s\n", out)
			cancel()
		} else {
			fmt.Printf("command returned:\n%s\n", out)
			cancel()
		}
	}
}

func (x *QemuCommand) Execute(args []string) error {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 100 * time.Second)
	fmt.Println("TAAAAAAAAA! ", opts.CPort, opts.CCountVM)
	/* XXX: give the process a chance to terminate. Proper waiting is
	 *  required here
	 */
	defer time.Sleep(time.Second)
	defer cancel()

	go qemu_run(ctx, cancel)

	var connection SshConnection

	for i := 0; i < opts.CCountVM; i++ {
		err := connection.Init(ctx, opts.CPort + i)
		if err != nil {
			return fmt.Errorf("Connection to VM on address[%d] failed: %w", opts.CPort + i, err)
		}
	}
	//fmt.Printf("connection established\n")
	return nil
}

func init() {
	parser.AddCommand("qemu",
		"Create and run VM in QEMU",
		"This command creates and starts a virtual machine in QEMU",
		&qemu_command)
}
