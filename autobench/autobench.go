package main

import (
	"fmt"
	"os"

	"github.com/jessevdk/go-flags"
	"github.com/zededa-yuri/nextgen-storage/autobench/pkg/mkconfig"
)

type Options struct {
	TimeOneTest        int 	  `short:"t" long:"time" description:"The time that each test will run in sec" default:"60"`
	SizeDiskGb		   int    `short:"s" long:"size" description:"The total size of file I/O for each thread of this job in Gb" default:"1"`
	OpType             string `short:"o" long:"optype" description:"Operation types I/O for fio config" default:"read,write"`
	BlockSize          string `short:"b" long:"bs" description:"Block size for fio config"  default:"4k,64k,1m"`
	Iodepth            string `short:"d" long:"iodepth" description:"Iodepth for fio config" default:"8,16,32"`
	CheckSumm          string `short:"c" long:"check" description:"Data integrity check. Can be one of the following values: (md5, crc64, crc32c, ..., sha256)"`
	Jobs               string `short:"j" long:"jobs" description:"Jobs for fio config" default:"1,8"`
	TargetFIODevice    string `short:"D" long:"targetdev" description:"[Optional] To specify block device as a target for FIO. Needs superuser rights (-u=root)."`
	LocalFolderResults string `short:"f" long:"folder" description:"[Optional] A name of folder with tests results" default:"FIOTestsResults"`
	LocalDirResults    string `short:"l" long:"localpath" description:"[Optional] Path to directory with test results"`
}

var opts Options
var parser = flags.NewParser(&opts, flags.Default)
var FioOptions = mkconfig.FioOptions{}

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

func InitFioOptions() error {
	if err := FioOptions.Operations.Set(opts.OpType); err != nil {
		return fmt.Errorf("fio tests failed: %v", err)
	}
	if err := FioOptions.BlockSize.Set(opts.BlockSize); err != nil {
		return fmt.Errorf("fio tests failed: %v", err)
	}
	if err := FioOptions.Jobs.Set(opts.Jobs); err != nil {
		return fmt.Errorf("fio tests failed: %v", err)
	}
	if err := FioOptions.Iodepth.Set(opts.Iodepth); err != nil {
		return fmt.Errorf("fio tests failed: %v", err)
	}

	FioOptions.SizeGb = opts.SizeDiskGb

	if opts.CheckSumm != "" {
		var valid = []string{"md5", "crc64", "crc32c", "crc32c-intel", "crc32", "crc16", "crc7", "xxhash", "sha512", "sha256", "sha1", "meta"}
		if mkconfig.Contains(valid, opts.CheckSumm) {
			return fmt.Errorf("invalid value for check summ")
		}
		FioOptions.CheckSumm = opts.CheckSumm
	}
	return nil
}

func main() {
	argparse()
}
