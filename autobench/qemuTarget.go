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
	"github.com/zededa-yuri/nextgen-storage/autobench/pkg/vhost"
	"github.com/zededa-yuri/nextgen-storage/autobench/qemutmp"
	"golang.org/x/crypto/ssh"
)

type QemuCommand struct {
	CQemuConfigDir string `short:"c" long:"config" description:"The option takes the path to the QEMU configuration file"`
	CFileLocation  string `short:"i" long:"image" description:"The option takes the path to the .img file" default:"bionic-server-cloudimg-i386.img"`
	CSizeDiskGb	   int    `short:"s" long:"size" description:"The total size for logical volume in Gb" default:"60"`
	CFormat        string `short:"f" long:"format" description:"Format options " default:"raw"`
	CVCpus         string `short:"v" long:"vcpu" description:"VCpu and core counts" default:"2"`
	CUser          string `short:"u" long:"user" description:"A user name for VM connections" default:"ubuntu"`
	CMemory        string `short:"m" long:"memory" description:"RAM memory value" default:"512"`
	CPassword      string `short:"x" long:"password" description:"Format options " default:"asdfqwer"`
	CPort          int    `short:"p" long:"port" description:"Port for connect to VM" default:"6666"`
	CCountVM       int    `short:"n" long:"vmcount" description:"Count create VM" default:"1"`
	CZfs           bool   `short:"z" long:"zfs" description:"Create zfs volume and share to vm via VHost"`
	CTargetDisk    string `short:"d" long:"disktarget" description:"Path to device for create zpool or lvm"`
	CLvm           bool   `short:"l" long:"lvm" description:"Create lvm volume and share to vm via VHost"`
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
	VhostWWPN    string // no set by default
}

type VirtM struct {
	ctx          context.Context
	cancel       context.CancelFunc
	sshClient    *ssh.Client
	timeOut      time.Duration
	port         int
	isRunning    bool
	imgPath      string
	userImg      string
	resultPath   string
	shareVolName string
	iblockId     string
	zfsDevice    string
	lvmDevice    string
	wwnAdress	 string
}

type VMlist []*VirtM

func writeMainConfig(path string, template_args VmConfig, zfsType bool) error {
	var curentTmp = qemutmp.QemuConfTemplate
	if zfsType {
		curentTmp = qemutmp.QemuConfVhostTemplate
	}

	t, err := template.New("qemu").Parse(curentTmp)
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

// qemuVmRun go-routine function with running VM
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
	qemuCmdFile, err := os.OpenFile(filepath.Join(vm.resultPath, "qemu-cmd.ini"),
		os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		fmt.Printf("failed to open file %s: %v\n", filepath.Join(vm.resultPath, "qemu-cmd.ini"), err)
	}
	defer qemuCmdFile.Close()

	if _, err := qemuCmdFile.WriteString(cmdStr); err != nil {
		fmt.Printf("failed write to file %s: %v\n", filepath.Join(vm.resultPath, "qemu-cmd.ini"), err)
	}

	var outbuf, errbuf bytes.Buffer
	cmd.Stdout = &outbuf
	cmd.Stderr = &errbuf

	err = cmd.Run() // This command will never ends
	if err != nil {
		fmt.Printf("QEMU VM message: %v; description=%v\n", err, ctx.Err())
		if outbuf.String() != "" {
			fmt.Println("Output:", outbuf.String(), errbuf.String())
		}
	}
	vm.cancel()
}

func (t *VMlist) AllocateVM(ctx context.Context, totalTime time.Duration) error {
	curentDate := time.Now().Format("2006-01-02-15:04:05")
	mainResultsDirForCurentTest := filepath.Join(getSelfPath(), "FIO-results-QEMU-Target"+curentDate)
	err := os.Mkdir(mainResultsDirForCurentTest, 0755)
	if err != nil {
		return fmt.Errorf("could not create local dir for result: %w", err)
	}

	log.Printf("Creating %d virtual machines\n", qemuCmd.CCountVM)

	for i := 0; i < qemuCmd.CCountVM; i++ {
		var vm VirtM
		vm.ctx, vm.cancel = context.WithTimeout(ctx, totalTime)
		vm.port = qemuCmd.CPort + i
		vm.timeOut = totalTime
		vm.shareVolName = fmt.Sprintf("vm%d", vm.port)
		vm.zfsDevice = filepath.Join("/dev/zvol/", "fiotest/", vm.shareVolName) // /dev/zvol/tank/test-zvol
		vm.lvmDevice = filepath.Join("/dev/", "fiotest/", vm.shareVolName)
		vm.iblockId = fmt.Sprintf("fiotest%d_iblock", vm.port)
		vm.userImg = filepath.Join(getSelfPath(), "user-data.img")
		vm.imgPath, err = getVMImage(i, qemuCmd.CFileLocation)

		if err != nil {
			return fmt.Errorf("create VM with adress localhost:%d failed! err:\n%v", vm.port, err)
		}

		if qemuCmd.CCountVM > 1 {
			vm.userImg = filepath.Join(getSelfPath(), fmt.Sprintf("%d-%s", i, "user-data.img"))
		}

		vm.resultPath = filepath.Join(mainResultsDirForCurentTest, fmt.Sprintf("vm-port-%d", vm.port))
		err = os.Mkdir(vm.resultPath, 0755)
		if err != nil {
			return fmt.Errorf("could not create local dir:[%s] for result: %w", vm.resultPath, err)
		}

		if qemuCmd.CZfs || qemuCmd.CLvm {
			FioOptions.SizeGb = qemuCmd.CSizeDiskGb - 1
			if qemuCmd.CZfs {
				if err := vhost.CreateZvol("fiotest", vm.shareVolName, qemuCmd.CSizeDiskGb); err != nil {
					return fmt.Errorf("create zvol:[%s] for VM with adress localhost:%d failed! err:\n%v",
						vm.shareVolName, vm.port, err)
				}
				vm.wwnAdress, err = vhost.SetupVhost(vm.zfsDevice, vm.iblockId)
			} else {
				if err := vhost.LVcreate(vm.shareVolName, "fiotest", qemuCmd.CSizeDiskGb); err != nil {
					return fmt.Errorf("create lvmVol:[%s] for VM with adress localhost:%d failed! err:\n%v",
						vm.shareVolName, vm.port, err)
				}
				vm.wwnAdress, err = vhost.SetupVhost(vm.lvmDevice, vm.iblockId)
			}
			if err != nil {
				return fmt.Errorf("create VHOST for vol:[%s] for VM with adress localhost:%d failed! err:\n%v",
					vm.shareVolName, vm.port, err)
			}

			templateArgs := VmConfig{
				FileLocation: qemuCmd.CFileLocation,
				Format:       qemuCmd.CFormat,
				VCpus:        qemuCmd.CVCpus,
				Memory:       qemuCmd.CMemory,
				Password:     qemuCmd.CPassword,
				VhostWWPN:    vm.wwnAdress,
			}
			if err := writeMainConfig(filepath.Join(vm.resultPath, "qemu.cfg"),
									  templateArgs, true); err != nil {
				// FIX ME del vhost and zvol or lv
				return fmt.Errorf("copy qemu vhost config to:[%s] failed! err:%v", vm.resultPath, err)
			}
		} else {
			templateArgs := VmConfig{
				FileLocation: qemuCmd.CFileLocation,
				Format:       qemuCmd.CFormat,
				VCpus:        qemuCmd.CVCpus,
				Memory:       qemuCmd.CMemory,
				Password:     qemuCmd.CPassword,
			}
			if err := writeMainConfig(filepath.Join(vm.resultPath, "qemu.cfg"),
									  templateArgs, false); err != nil {
				return fmt.Errorf("copy qemu config to:[%s] failed! err:%v", vm.resultPath, err)
			}
		}

		go qemuVmRun(vm.ctx, vm, filepath.Join(vm.resultPath, "qemu.cfg"))

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
				return fmt.Errorf("create VM with adress localhost:%d failed! err:\n%v",
					vm.port, vm.ctx.Err())
			}
			time.Sleep(3 * time.Second)
		}

		if !vm.isRunning {
			log.Printf("unable to connect: localhost:%d err:%v", vm.port, err)
		}

		if err != nil {
			vm.cancel()
			for _, vmo := range *t {
				vmo.cancel()
			}
			// FIX ME del vhost and zvol or lv
			return fmt.Errorf("create VM with adress localhost:%d failed! err:%v", vm.port, err)
		}

		*t = append(*t, &vm)
	}

	return nil
}

func (t VMlist) FreeVM() {
	for _, vm := range t {
		vm.sshClient.Close()
		vm.cancel()
		//If we have only one VM we shouldn't delete img`s
		if qemuCmd.CCountVM > 1 {
			if err := os.Remove(vm.imgPath); err != nil {
				log.Printf("Remove %s failed! err:%v", vm.imgPath, err)
			}
			if err := os.Remove(vm.userImg); err != nil {
				log.Printf("Remove %s failed! err:%v", vm.imgPath, err)
			}
		}

		if qemuCmd.CZfs || qemuCmd.CLvm {
			if err := vhost.VHostDeleteIBlock(vm.wwnAdress); err != nil {
				log.Printf("Remove VHOST wwn: %s failed! err:%v", vm.wwnAdress, err)
			}
			if err := vhost.TargetDeleteIBlock(vm.iblockId); err != nil {
				log.Printf("Remove Target: %s failed! err:%v", vm.iblockId, err)
			}
			if qemuCmd.CZfs {
				if err := vhost.DestroyZvol("fiotest", vm.shareVolName); err != nil {
					log.Printf("Remove zvol: %s failed! err:%v", vm.shareVolName, err)
				}
			} else {
				if err := vhost.LVremove(vm.shareVolName, "fiotest"); err != nil {
					log.Printf("LVremove %s failed err:%v", vm.shareVolName, err)
				}
			}
		}
	}
}

// fio - go-routine function with ssh
// connect and running fio comand on VM
func fio(virt *VirtM, localResultsFolder,
	targetDevice string, fioOptions mkconfig.FioOptions,
	fioTestTime time.Duration) {
	// FIX ME change target device if we have zfs option
	if err := fiotests.RunFIOTest(virt.sshClient, qemuCmd.CUser, localResultsFolder,
		virt.resultPath, targetDevice, fioOptions,
		fioTestTime); err != nil {
		log.Printf("FIO tests failed on VM [%s]: error: %v",
			fmt.Sprintf("localhost:%d", virt.port), err)
		testFailed <- true
	}
	log.Printf("Test on a VM with port: %d finished! Wait for VM to complete.", virt.port)
}

// RunCommand - Starts the testing process for qemu target
func RunCommand(ctx context.Context, virtM VMlist) error {
	if err := InitFioOptions(); err != nil {
		return fmt.Errorf("error get fio params: %w", err)
	}

	var countTests = mkconfig.CountTests(FioOptions)
	const bufferTime = 3 * time.Minute
	var totalTime = time.Duration(int64(countTests)*int64(time.Duration(opts.TimeOneTest) * time.Second) + int64(bufferTime))

	if qemuCmd.CZfs && qemuCmd.CTargetDisk != "" {
		if err := vhost.CheckZfsOnSystem(); err != nil {
			return fmt.Errorf("ZFS not found: %v", err)
		}
		if err := vhost.CreateZpool("fiotest", qemuCmd.CTargetDisk); err != nil {
			return fmt.Errorf("Create zpool failed: %v", err)
		}
	}

	if qemuCmd.CLvm && qemuCmd.CTargetDisk != "" {
		if err := vhost.CheckLvmOnSystem(); err != nil {
			return fmt.Errorf("LVM not found: %v", err)
		}
		if err := vhost.PVcreate(qemuCmd.CTargetDisk); err != nil {
			return fmt.Errorf("PVcreate failed: %v", err)
		}
		if err := vhost.VGcreate(qemuCmd.CTargetDisk, "fiotest"); err != nil {
			return fmt.Errorf("VGcreate failed: %v", err)
		}
	}

	ctxVMs, cancelVMS := context.WithTimeout(ctx, totalTime)
	err := virtM.AllocateVM(ctxVMs, totalTime)
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
			time.Duration(opts.TimeOneTest) * time.Second,
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
	if qemuCmd.CZfs {
		if err := vhost.DestroyZpool("fiotest"); err != nil {
			fmt.Println("Destroy zpool failed", err)
		}
	}
	if qemuCmd.CLvm {
		if err := vhost.DestroyLvm(qemuCmd.CTargetDisk, "fiotest"); err != nil {
			fmt.Println("Destroy zpool failed", err)
		}
	}
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
