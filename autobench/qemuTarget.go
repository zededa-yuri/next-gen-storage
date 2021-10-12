package main

import (
	"bytes"
	"context"
	"fmt"

	"log"
	"os"
	"os/exec"
	"path/filepath"
	"text/template"
	"time"

	"github.com/zededa-yuri/nextgen-storage/autobench/pkg/fiotests"
	"github.com/zededa-yuri/nextgen-storage/autobench/pkg/mkconfig"
	"github.com/zededa-yuri/nextgen-storage/autobench/qemutmp"
	"golang.org/x/crypto/ssh"
	zfs "github.com/bicomsystems/go-libzfs"
)

type QemuCommand struct {
	CQemuConfigDir string `short:"c" long:"config" description:"The option takes the path to the QEMU configuration file"`
	CFileLocation  string `short:"i" long:"image" description:"The option takes the path to the .img file" default:"bionic-server-cloudimg-i386.img"`
	CFormat        string `short:"f" long:"format" description:"Format options " default:"raw"`
	CVCpus         string `short:"v" long:"vcpu" description:"VCpu and core counts" default:"2"`
	CUser		   string `short:"u" long:"user" description:"A user name for VM connections" default:"ubuntu"`
	CMemory        string `short:"m" long:"memory" description:"RAM memory value" default:"512"`
	CPassword      string `short:"x" long:"password" description:"Format options " default:"asdfqwer"`
	CPort          int    `short:"p" long:"port" description:"Port for connect to VM" default:"6666"`
	CCountVM       int    `short:"n" long:"number" description:"Count create VM" default:"1"`
}

var qemuCmd QemuCommand
var testFailed = make(chan bool)

type VmConfig struct {
	FileLocation string // default "bionic-server-cloudimg-i386.img"
	Format       string // default "raw"
	VCpus        string // default "2"
	Memory       string // default "512"
	Kernel       string // default ""
	Password     string // default "asdfqwer"
}

type VirtM struct {
	ctx       	context.Context
	cancel    	context.CancelFunc
	sshClient 	*ssh.Client
	timeOut   	time.Duration
	port      	int
	isRunning 	bool
	imgPath   	string
	userImg   	string
	resultPatch string
}

type VMlist []*VirtM

func writeMainConfig(path string, template_args VmConfig) error {
	t, err := template.New("qemu").Parse(qemutmp.QemuConfTemplate)
	if err != nil {
		fmt.Printf("failed parse template%v\n", err)
		return err
	}

	configFile, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)

	if err != nil {
		fmt.Printf("failed to open file %s: %v\n", path, err)
		return err
	}
	defer configFile.Close()

	if err = t.Execute(configFile, template_args); err != nil {
		fmt.Println("cant parse template")
		return err
	}

	return nil
}

func getSelfPath() string {
	ex, err := os.Executable()
	if err != nil {
		panic(err)
	}
	return filepath.Dir(ex)
}

func getVMImage(index int, filename string) (string, error) {
	if _, err := os.Stat(filepath.Join(getSelfPath(), filename)); err != nil {
		if os.IsNotExist(err) {
			return "", err
		}
	}

	defPathImg := filepath.Join(getSelfPath(), filename)
	if qemuCmd.CCountVM <= 1 {
		return defPathImg, nil
	}

	fPath := filepath.Join(getSelfPath(), fmt.Sprintf("%d-%s", index, filename))
	_, err := exec.Command("cp", defPathImg, fPath).CombinedOutput()
	if err != nil {
		fmt.Printf("Run command cp %s -> %s  failed: err %v", defPathImg, fPath, err)
		return "", err
	}

	userDPathDef := filepath.Join(getSelfPath(), "user-data.img")
	userDPath := filepath.Join(getSelfPath(), fmt.Sprintf("%d-%s", index, "user-data.img"))
	_, err = exec.Command("cp", userDPathDef, userDPath).CombinedOutput()
	if err != nil {
		fmt.Printf("Run command cp %s -> %s  failed: err %v", userDPathDef, userDPath, err)
		return "", err
	}

	return fPath, nil
}

func qemuVmRun(ctx context.Context, vm VirtM, qemuConfigDir string) {
	cmd := exec.CommandContext(ctx,
		"qemu-system-x86_64",
		"-cpu", "host",
		"-readconfig", qemuConfigDir,
		"-drive", fmt.Sprintf("file=%s,format=raw,if=none,id=hd", vm.imgPath),
		"-display", "none",
		"-cdrom", vm.userImg,
		"-device", "e1000,netdev=net0", "-netdev", fmt.Sprintf("user,id=net0,hostfwd=tcp::%d-:22", vm.port),
		"-serial", "chardev:ch0")

	cmdStr := cmd.String()
	qemuCmdFile, err := os.OpenFile(filepath.Join(vm.resultPatch, "qemu-cmd.ini"),
									os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		fmt.Printf("failed to open file %s: %v\n", filepath.Join(vm.resultPatch, "qemu-cmd.ini"), err)
	}
	defer qemuCmdFile.Close()

	if _, err := qemuCmdFile.WriteString(cmdStr); err != nil {
		fmt.Printf("failed write to file %s: %v\n", filepath.Join(vm.resultPatch, "qemu-cmd.ini"), err)
	}

	var outbuf, errbuf bytes.Buffer
	cmd.Stdout = &outbuf
	cmd.Stderr = &errbuf

	err = cmd.Run() // This command will never ends
	if err != nil {
		fmt.Printf("QEMU VM message: %v; description=%v\n", err, ctx.Err())
		if (outbuf.String() != "") {
			fmt.Println("Output:", outbuf.String(), errbuf.String())
		}
	}
	vm.cancel()
}

func (t *VMlist) AllocateVM(ctx context.Context, totalTime time.Duration) error {
	templateArgs := VmConfig{
		FileLocation: qemuCmd.CFileLocation,
		Format:       qemuCmd.CFormat,
		VCpus:        qemuCmd.CVCpus,
		Memory:       qemuCmd.CMemory,
		Password:     qemuCmd.CPassword,
	}

	qemuConfigDir := filepath.Join(getSelfPath(), "qemu.cfg")
	err := writeMainConfig(qemuConfigDir, templateArgs)
	if err != nil {
		return fmt.Errorf("create qemu config failed! err:%v", err)
	}

	curentDate := time.Now().Format("2006-01-02-15:04:05")
	mainResultsDirForCurentTest := filepath.Join(getSelfPath(), "FIO-results-QEMU-Target" + curentDate)
	err = os.Mkdir(mainResultsDirForCurentTest, 0755)
	if err != nil {
		return fmt.Errorf("could not create local dir for result: %w", err)
	}

	log.Printf("Creating %d virtual machines\n", qemuCmd.CCountVM)

	for i := 0; i < qemuCmd.CCountVM; i++ {
		var vm VirtM
		vm.ctx, vm.cancel = context.WithTimeout(ctx, totalTime)
		vm.port = qemuCmd.CPort + i
		vm.timeOut = totalTime
		vm.userImg = filepath.Join(getSelfPath(), "user-data.img")
		vm.imgPath, err = getVMImage(i, qemuCmd.CFileLocation)
		if err != nil {
			return fmt.Errorf("create VM with adress localhost:%d failed! err:\n%v", vm.port, err)
		}

		if qemuCmd.CCountVM > 1 {
			vm.userImg = filepath.Join(getSelfPath(), fmt.Sprintf("%d-%s", i, "user-data.img"))
		}

		vm.resultPatch = filepath.Join(mainResultsDirForCurentTest, fmt.Sprintf("vm-port-%d", vm.port))
		err = os.Mkdir(vm.resultPatch, 0755)
		if err != nil {
			return fmt.Errorf("could not create local dir:[%s] for result: %w", vm.resultPatch, err)
		}

		go qemuVmRun(vm.ctx, vm, qemuConfigDir)

		config := &ssh.ClientConfig{
			User: qemuCmd.CUser,
			Auth: []ssh.AuthMethod{
				ssh.Password(qemuCmd.CPassword),
			},
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		}

		const tryTimes = 30
		for i := 0; i < tryTimes; i++ {
			vm.sshClient, err = ssh.Dial("tcp", fmt.Sprintf("localhost:%d", vm.port), config)
			if err != nil {
				vm.isRunning = false
			} else {
				log.Printf("VM creation and connection to: localhost:%d was successful", vm.port)
				vm.isRunning = true
				break
			}
			if vm.ctx.Err() == context.Canceled || vm.ctx.Err() == context.DeadlineExceeded {
				return fmt.Errorf("create VM with adress localhost:%d failed! err:\n%v", vm.port, vm.ctx.Err())
			}
			time.Sleep(3 * time.Second)
		}

		if (!vm.isRunning) {
			log.Printf("unable to connect: localhost:%d err:%v", vm.port, err)
		}

		if err != nil {
			vm.cancel()
			for _, vmo := range *t {
				vmo.cancel()
			}
			return fmt.Errorf("create VM with adress localhost:%d failed! err:%v", vm.port, err)
		}

		err := writeMainConfig(filepath.Join(vm.resultPatch, "qemu.cfg"), templateArgs)
		if err != nil {
			return fmt.Errorf("copy qemu config to:[%s] failed! err:%v", vm.resultPatch, err)
		}

		*t = append(*t, &vm)
	}

	return nil
}

func (t VMlist) FreeVM() {
	for _, vm := range t {
		vm.sshClient.Close()
		vm.cancel()
		if err := os.Remove(vm.imgPath); err != nil {
			log.Printf("Remove %s failed! err:%v", vm.imgPath, err)
		}
		if err := os.Remove(vm.userImg); err != nil {
			log.Printf("Remove %s failed! err:%v", vm.imgPath, err)
		}
	}
}

func fio(virt *VirtM, localResultsFolder,
	targetDevice string, fioOptions mkconfig.FioOptions,
	fioTestTime time.Duration) {
	if err := fiotests.RunFIOTest(virt.sshClient, qemuCmd.CUser, localResultsFolder,
		virt.resultPatch, targetDevice, fioOptions,
		fioTestTime); err != nil {
		log.Printf("FIO tests failed on VM [%s]: error: %v", fmt.Sprintf("localhost:%d", virt.port), err)
		testFailed <- true
	}
	log.Printf("Test on a VM with port: %d finished! Wait for VM to complete.", virt.port)
}

func SetupDiskZfs(ctx context.Context, target string) error {
	/* TODO: use go-libzfs package to create pool */
	cmd := exec.Command("zpool",
		"create",
		"tank",
		"-f",
		target)

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed creating zfs pool: %v", err)
	}

	pool, err := zfs.PoolOpen("tank")
	if err != nil {
		return fmt.Errorf("failed creating zfs pool: %v", err)
	}

	defer pool.Close()
	return nil
}

func SetupDiskZfs(ctx context.Context, target string) error {
	/* TODO: use go-libzfs package to create pool */
	cmd := exec.Command("zpool",
		"create",
		"tank",
		"-f",
		target)

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed creating zfs pool: %v", err)
	}

	pool, err := zfs.PoolOpen("tank")
	if err != nil {
		return fmt.Errorf("failed creating zfs pool: %v", err)
	}

	defer pool.Close()
	return nil
}

func SetupDiskZfs(ctx context.Context, target string) error {
	/* TODO: use go-libzfs package to create pool */
	cmd := exec.Command("zpool",
		"create",
		"tank",
		"-f",
		target)

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed creating zfs pool: %v", err)
	}

	pool, err := zfs.PoolOpen("tank")
	if err != nil {
		return fmt.Errorf("failed creating zfs pool: %v", err)
	}

	defer pool.Close()
	return nil
}

func RunCommand(ctx context.Context, virtM VMlist) error {
	err := InitFioOptions()
	if err != nil {
		return fmt.Errorf("error get fio params: %w", err)
	}

	var countTests = mkconfig.CountTests(FioOptions)
	const bufferTime = 3 * time.Minute
	var totalTime = time.Duration(int64(countTests)*int64(60*time.Second) + int64(bufferTime))
	ctxVMs, cancelVMS := context.WithTimeout(ctx, totalTime)

	err = SetupDiskZfs(ctxVMs, opts.TargetDisk)
	if err != nil {
		cancelVMS()
		return fmt.Errorf("Can't setup disk %s:%v", opts.TargetDisk, err)
	}

	err = virtM.AllocateVM(ctxVMs, totalTime)
	if err != nil {
		cancelVMS()
		return fmt.Errorf("VM create in QEMU failed err:%v", err)
	}

	for _, vm := range virtM {
		time.Sleep(5 * time.Second) // For create new folder for new test with other name
		go fio(
			vm,
			opts.LocalFolderResults,
			opts.TargetFIODevice,
			FioOptions,
			60*time.Second,
		)
	}

	// Heartbeat
	fmt.Println("Total generated tests:", countTests)
	fmt.Println("Total waiting time before the end of the test:", totalTime)
	timerTomeOut := time.After(totalTime)
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

there:
	for {
		select {
		case <-timerTomeOut:
			ticker.Stop()
			break there
		case <-testFailed:
			ticker.Stop()
			break there

		}
	}

	fmt.Println("All FIO tests finished!")
	virtM.FreeVM()
	cancelVMS()
	return nil
}

func (x *QemuCommand) Execute(args []string) error {
	ctx := context.Background()
	var virtM = make(VMlist, 0)
	err := RunCommand(ctx, virtM)
	if err != nil {
		return fmt.Errorf("qemu test failed: %v", err)
	}

	return nil
}

func init() {
	parser.AddCommand(
		"qemu",
		"Create and run VM with FIO test in QEMU",
		"This command creates and starts a virtual machine with FIO tests in QEMU",
		&qemuCmd,
	)
}
