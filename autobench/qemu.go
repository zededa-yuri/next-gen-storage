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

type VirtM struct {
	ctx 		context.Context
	cancel 		context.CancelFunc
	sshClient 	*ssh.Client
	timeOut 	time.Duration
	port 		int
	status 		bool
	imgPath 	string
	userImg		string
}

type VMlist []VirtM

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

func get_self_path() (string) {
	ex, err := os.Executable()
	if err != nil {
		panic(err)
	}
	return filepath.Dir(ex)
}


func getVMImage(index int, filename string) (string, error) {
    if _, err := os.Stat(filepath.Join(get_self_path(), filename)); err != nil {
        if os.IsNotExist(err) {
			return "", err
        }
    }

	defPathImg := filepath.Join(get_self_path(), filename)
	if opts.CCountVM <=1 {
		return defPathImg, nil
	}

	fPath := filepath.Join(get_self_path(), fmt.Sprintf("%d-%s", index, filename))
	_, err := exec.Command("cp", defPathImg, fPath).CombinedOutput()
	if err != nil {
		fmt.Printf("Run command cp %s -> %s  failed: err %v", defPathImg, fPath, err)
		return "", err
	}

	userDPathDef := filepath.Join(get_self_path(), "user-data.img")
	userDPath := filepath.Join(get_self_path(), fmt.Sprintf("%d-%s", index, "user-data.img"))
	_, err = exec.Command("cp", userDPathDef, userDPath).CombinedOutput()
	if err != nil {
		fmt.Printf("Run command cp %s -> %s  failed: err %v", userDPathDef, userDPath, err)
		return "", err
	}

    return fPath, nil
}

func qemuVmRun(vm VirtM, qemuConfigDir string) {
	cmd := exec.CommandContext(vm.ctx,
		"qemu-system-x86_64",
		"-hda", vm.imgPath,
		"-cpu", "host",
		"-readconfig",  qemuConfigDir,
		"-display", "none",
		"-drive", fmt.Sprintf("file=%s,format=raw", vm.userImg),
		"-device", "e1000,netdev=net0",  "-netdev",  fmt.Sprintf("user,id=net0,hostfwd=tcp::%d-:22", vm.port),
		"-serial", "chardev:ch0")

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	err := cmd.Run() //no have end for this command
	if err != nil {
		fmt.Printf("error launching command: %v; err=%v\nStdout:\n%s\n", err, vm.ctx.Err(), out)
	}
	vm.cancel()
}

func (t* VMlist) AllocateVM(ctx context.Context, totalTime time.Duration) error {
	*t = []VirtM{}
	template_args := VmConfig{}
	template_args.FileLocation = qemu_command.CFileLocation
	template_args.Format = qemu_command.CFormat
	template_args.VCpus = qemu_command.CVCpus
	template_args.Memory = qemu_command.CMemory
	template_args.Password = qemu_command.CPassword

	qemuConfigDir := filepath.Join(get_self_path(), "qemu.cfg")
	err := write_main_config(qemuConfigDir, template_args)
	if err != nil {
		return fmt.Errorf("create qemu config failed! err:%v", err)
	}

	log.Printf("Creating %d virtual macnines\n", opts.CCountVM)

	for i := 0; i < opts.CCountVM; i++ {
		var vm VirtM
		vm.ctx, vm.cancel = context.WithTimeout(ctx, totalTime)
		vm.port = opts.CPort + i
		vm.timeOut = totalTime
		vm.userImg = filepath.Join(get_self_path(), "user-data.img")
		vm.imgPath, err = getVMImage(i, qemu_command.CFileLocation)
		if err != nil {
			return fmt.Errorf("create VM with adress localhost:%d failed! err:\n%v", vm.port, err)
		}
		if opts.CCountVM > 1 {
			vm.userImg = filepath.Join(get_self_path(), fmt.Sprintf("%d-%s", i, "user-data.img"))
		}

		go qemuVmRun(vm, qemuConfigDir)

		config := &ssh.ClientConfig{
			User: "ubuntu",
			Auth: []ssh.AuthMethod{
				ssh.Password(qemu_command.CPassword),
			},
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		}

		for i := 0; i < 30; i++ {
			vm.sshClient, err = ssh.Dial("tcp", fmt.Sprintf("localhost:%d", vm.port), config)
			if err != nil {
				log.Printf("Unable to connect: localhost:%d err:%v", vm.port, err)
			} else {
				log.Printf("Connection to: localhost:%d was successful", vm.port)
				vm.status = true
				break
			}
			if vm.ctx.Err() == context.Canceled || vm.ctx.Err() == context.DeadlineExceeded {
				return fmt.Errorf("create VM with adress localhost:%d failed! err:\n%v", vm.port, vm.ctx.Err())
			}
			time.Sleep(3 * time.Second)
		}

		if err != nil {
			vm.cancel()
			for _, vmo := range *t {
				vmo.cancel()
			}
			return fmt.Errorf("create VM with adress localhost:%d failed! err:%v", vm.port, err)
		}

		*t = append(*t, vm) //update list with VM
	}

	return nil
}

func (t* VMlist) FreeVM(vmList VMlist) {
	if len(vmList) != 0 {
		for _, vm := range vmList {
			vm.cancel() //not work
		}
	}
}

func (x *QemuCommand) Execute(args []string) error {

	return nil
}

func init() {
	parser.AddCommand("qemu",
		"Create and run VM in QEMU",
		"This command creates and starts a virtual machine in QEMU",
		&qemu_command)
}
