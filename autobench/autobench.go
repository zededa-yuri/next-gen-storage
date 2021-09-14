package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/jessevdk/go-flags"
	"github.com/zededa-yuri/nextgen-storage/autobench/pkg/fiotests"
	"github.com/zededa-yuri/nextgen-storage/autobench/pkg/mkconfig"
)


const fioComandDesc = `
	This command allows you to run FIO testing via an ssh client.
	This command can take configuration values for FIO.

	Example use:
	autobench -p=6666 -n=2 fio -T=qemu -a=127.0.0.1 -u=ubuntu -o=read,randread -b=4k,1m -t=30

`
type Options struct {
	CPort			int	`short:"p" long:"port" description:"Port for connect to VM" default:"6666"`
	CCountVM		int `short:"n" long:"number" description:"Count create VM" default:"1"`
}

type FioParametrs struct {
	SSHhost string 				`short:"a" long:"adress" description:"ip:port for ssh connections." default:"127.0.0.1"`
	SSHUser string 				`short:"u" long:"user" description:"A user name for ssh connections" default:"ubuntu"`
	TimeOneTest int 			`short:"t" long:"time" description:"The time that each test will run in sec" default:"60"`
	OpType string				`short:"o" long:"optype" description:"Operation types I/O for fio config" default:"read,write"`
	BlockSize string 			`short:"b" long:"bs" description:"Block size for fio config"  default:"4k,64k,1m"`
	Iodepth string 				`short:"d" long:"iodepth" description:"Iodepth for fio config" default:"8,16,32"`
	CheckSumm string			`short:"c" long:"check" description:"Data integrity check. Can be one of the following values: (md5, crc64, crc32c, ..., sha256)"`
	Jobs string					`short:"j" long:"jobs" description:"Jobs for fio config" default:"1,8"`
	TargetFIODevice string 		`short:"D" long:"targetdev" description:"[Optional] To specify block device as a target for FIO. Needs superuser rights (-u=root)."`
	LocalFolderResults string 	`short:"f" long:"folder" description:"[Optional] A name of folder with tests results" default:"FIOTestsResults"`
	LocalDirResults string 		`short:"p" long:"path" description:"[Optional] Path to directory with test results"`
	Target string				`short:"T" long:"target" description:"Target for benchmark tests"`
}


var fioCmd FioParametrs
var opts Options
var parser = flags.NewParser(&opts, flags.Default)
var testFailed = make(chan bool)

func argparse() {
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
}

func fio(port int, sshHost, sshUser, localResultsFolder, localDirResults, targetDevice string, fioOptions mkconfig.FioOptions, fioTestTime time.Duration) {
	if err := fiotests.RunFIOTest(fmt.Sprintf("%s:%d", sshHost, port), sshUser, localResultsFolder, localDirResults, targetDevice, fioOptions, fioTestTime); err != nil {
		log.Printf("FIO tests failed on VM [%s]: error: %v", fmt.Sprintf("%s:%d", sshHost, port), err)
		testFailed <- true
	}
}

// Execute FIO tests via ssh
func (x *FioParametrs) Execute(args []string) error {
	ctx := context.Background()
	var fioOptions = mkconfig.FioOptions{}

	err := fioOptions.Operations.Set(fioCmd.OpType)
	if err != nil {
		return fmt.Errorf("FIO tests failed: %v", err)
	}

	err = fioOptions.BlockSize.Set(fioCmd.BlockSize)
	if err != nil {
		return fmt.Errorf("FIO tests failed: %v", err)
	}

	err = fioOptions.Jobs.Set(fioCmd.Jobs)
	if err != nil {
		return fmt.Errorf("FIO tests failed: %v", err)
	}

	err = fioOptions.Iodepth.Set(fioCmd.Iodepth)
	if err != nil {
		return fmt.Errorf("FIO tests failed: %v", err)
	}

	if fioCmd.CheckSumm != "" {
		var valid = []string{"md5", "crc64", "crc32c", "crc32c-intel", "crc32", "crc16", "crc7", "xxhash", "sha512", "sha256", "sha1", "meta"}
		if mkconfig.Contains(valid, fioCmd.CheckSumm) {
			return fmt.Errorf("invalid value for check summ")
		}
		fioOptions.CheckSumm = fioCmd.CheckSumm
	}

	var countTests = mkconfig.CountTests(fioOptions)
	const bufferTime = 5 * time.Minute
	var totalTime = time.Duration(int64(countTests) * int64(60 * time.Second) + int64(bufferTime))
	ctxVM, cancel := context.WithTimeout(ctx, totalTime)
	defer cancel()

	if fioCmd.Target == "qemu" {
		go qemu_command.CreateQemuVM(ctxVM, cancel, totalTime, args)
	}
	time.Sleep(30 * time.Second) // For waiting create VM

	for i := 0; i < opts.CCountVM; i++ {
		time.Sleep(5 * time.Second) // For create new folder for new test
		go fio(opts.CPort + i, fioCmd.SSHhost, fioCmd.SSHUser, fioCmd.LocalFolderResults, fioCmd.LocalDirResults, fioCmd.TargetFIODevice, fioOptions, 60 * time.Second)
	}

	// Heartbeat
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
			cancel()
			ticker.Stop()
			break there
		}
	}
	return nil
}

func init() {
	parser.AddCommand("fio", "Run FIO testing via an ssh client. Use the fio --help command for more information.", fioComandDesc, &fioCmd)
}

func main() {
	argparse()
}
