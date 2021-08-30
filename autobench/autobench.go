package main

import (
	"os"
	"fmt"
	"time"
	"github.com/jessevdk/go-flags"
	"github.com/zededa-yuri/nextgen-storage/autobench/pkg/mkconfig"
	"github.com/zededa-yuri/nextgen-storage/autobench/pkg/fiotests"
)

type Options struct {
	//Verbose []bool `short:"v" long:"verbose" description:"Show verbose debug information"`
}

type FioParametrs struct {
	SSHhost string `short:"a" long:"adress" description:"ip:port for ssh connections." default:"127.0.0.1:22"`
	SSHUser string `short:"u" long:"user" description:"A user name for ssh connections" default:"root"`
	LocalDirResults string `short:"d" long:"dir" description:"A name of directory with tests results" default:"FIOTestsResults"`
	//TimeOneTest int `short:"t" long:"time" description:"The time that each test will run in sec" default:"60"`
}

var fioCmd FioParametrs
var opts Options
var parser = flags.NewParser(&opts, flags.Default)

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

// Execute FIO tests via ssh
func (x *FioParametrs) Execute(args []string) error {
	var fioOptions = mkconfig.FioOptions{}
	fmt.Printf("SSHhost: [%s]\n", fioCmd.SSHhost)
	fmt.Printf("SSHUser: [%s]\n", fioCmd.SSHUser)
	fmt.Printf("LocalDirResults: [%s]\n", fioCmd.LocalDirResults)
	//fmt.Printf("TimeOneTest: [%d]\n", fioCmd.TimeOneTest)
	if err := fiotests.RunFIOTest(fioCmd.SSHhost, fioCmd.SSHUser, fioCmd.LocalDirResults, fioOptions, 60 * time.Second); err != nil {
		return fmt.Errorf("FIO tests failed: %v", err)
	}

	return nil
}

func init() {
	parser.AddCommand("fio", "This command allows you to run FIO testing via an ssh client. Can take configuration values. Use the fio --help command for more information.", "AHAHAHAHA", &fioCmd)
}

func main() {
	argparse()
}
