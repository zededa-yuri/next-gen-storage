package main

import (
	"fmt"
	"time"

	"github.com/zededa-yuri/nextgen-storage/autobench/pkg/fiotests"
)

type LocalCommand struct {
	User string `short:"u" long:"user" description:"A user name for save results in home directory"`
}

var localCmd LocalCommand

func init() {
	parser.AddCommand(
		"local",
		"Create test and run on local machine",
		"This command starts Test on local machine",
		&localCmd,
	)
}

func (x LocalCommand) Execute(args []string) error {
	err := InitFioOptions()
	if err != nil {
		return fmt.Errorf("error get fio params: %w", err)
	}

	err = fiotests.RunFIOTestLocal(localCmd.User, opts.LocalFolderResults, opts.LocalDirResults, opts.TargetFIODevice, FioOptions, 60*time.Second)
	if err != nil {
		return fmt.Errorf("fio tests failed error: %v", err)
	}

	return nil
}
