package main

import (
	"bytes"
	"context"
	"fmt"

	//	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"text/template"
	"time"

	"github.com/zededa-yuri/nextgen-storage/autobench/qemutmp"
	"golang.org/x/crypto/ssh"
)


type QemuCommand struct {
	CQemuConfigDir 	string `short:"c" long:"config" description:"The option takes the path to the QEMU configuration file"`
	CFileLocation 	string `short:"i" long:"image" description:"The option takes the path to the .img file" default:"bionic-server-cloudimg-i386.img"`
	CFormat 		string `short:"f" long:"format" description:"Format options " default:"raw"`
	CVCpus 			string `short:"v" long:"vcpu" description:"VCpu and core counts" default:"2"`
	CMemory			string `short:"m" long:"memory" description:"RAM memory value" default:"512"`
	CPassword		string `short:"x" long:"password" description:"Format options " default:"asdfqwer"`
}

var qemu_command QemuCommand

type VmConfig struct {
	FileLocation 	string // default "bionic-server-cloudimg-i386.img"
	Format 			string // default "raw"
	VCpus 			string // default "2"
	Memory 			string // default "512"
	Kernel 			string // default ""
	Password		string // default "asdfqwer"
}

type Vm struct {
	ctx context.Context
	cancel context.CancelFunc
	ssh *ssh.Client
	timeOut time.Duration
	ip string
	port int
	status bool
	imgpath string
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
	var err error

	config := &ssh.ClientConfig{
		User: "ubuntu",
		Auth: []ssh.AuthMethod{
			// Use the PublicKeys method for remote authentication.
			//ssh.PublicKeys(signer),
			ssh.Password("asdfqwer"),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

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


func Exists(filename string) bool {
    if _, err := os.Stat(filename); err != nil {
        if os.IsNotExist(err) {
            return false
        }
    }
    return true
}

func qemu_run(ctx context.Context, cancel context.CancelFunc, qemuConfigDir string, port int) {

	cmd := exec.CommandContext(ctx,
		"qemu-system-x86_64",
		"-hda", "bionic-server-cloudimg-i386.img",
		"-cpu", "host",
		"-readconfig",  qemuConfigDir,
		"-display", "none",
		"-drive", "file=user-data.img,format=raw",
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

func (x *QemuCommand) CreateQemuVM(ctx context.Context, cancel context.CancelFunc, timeWork time.Duration, args []string) {
	template_args := VmConfig{}
	template_args.FileLocation = qemu_command.CFileLocation
	template_args.Format = qemu_command.CFormat
	template_args.VCpus = qemu_command.CVCpus
	template_args.Memory = qemu_command.CMemory
	template_args.Password = qemu_command.CPassword

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

	var connection SshConnection

	// FIX ME FIRST CONNECTION FOR KNOWN HOSTS
	time.Sleep(3 * time.Second) // For waiting create VM
	for i := 0; i < opts.CCountVM; i++ {
		err := connection.Init(ctx, opts.CPort + i)
		if err != nil {
			fmt.Println("connection to VM on address failed:", opts.CPort + i, err)
		}
	}

}

func (x *QemuCommand) Execute(args []string) error {
	ctx := context.Background()
	ctxVM, cancel := context.WithTimeout(ctx, 60 * time.Minute)
	defer cancel()
	qemu_command.CreateQemuVM(ctxVM, cancel, 60 * time.Minute, args)

	//need waiting system

	return nil
}

func init() {
	parser.AddCommand("qemu",
		"Create and run VM in QEMU",
		"This command creates and starts a virtual machine in QEMU",
		&qemu_command)
}
