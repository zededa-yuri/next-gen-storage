package main

import (
	"bytes"
	"fmt"
	"os/exec"
)

type QemuCommand struct {
	Gdb bool `short:"g" long:"gdb" description:"just at test argurment"`
	Verbose []bool `short:"v" long:"verbose" description:"Show verbose debug information"`
}

var qemu_command QemuCommand

func (x *QemuCommand) Execute(args []string) error {
	fmt.Printf("Calling command, gdb=%v, verbose=%v, args are %s\n", x.Gdb, x.Verbose, args)

	cmd := exec.Command("ls", "-lah")
	var out bytes.Buffer
	cmd.Stdout = &out

	fmt.Printf("Running a command\n")
	err := cmd.Run()

	if err != nil {
		return err
	}

	fmt.Printf("command returned:\n%s\n", out)
	return nil
}

func init() {
	fmt.Printf("--- %v\n", qemu_command.Gdb)

	parser.AddCommand("qemu",
		"Run benchmark in qemu",
		"Long description",
		&qemu_command)
}
