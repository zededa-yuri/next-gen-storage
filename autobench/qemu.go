package main

import (
	"fmt"
)

type QemuCommand struct {
	Gdb bool `short:"g" long:"gdb" description:"just at test argurment"`
	Verbose []bool `short:"v" long:"verbose" description:"Show verbose debug information"`
}

var qemu_command QemuCommand

func (x *QemuCommand) Execute(args []string) error {
	fmt.Printf("Calling command, gdb=%v, verbose=%v, args are %s\n", x.Gdb, x.Verbose, args)
	return nil
}

func init() {
	fmt.Printf("--- %v\n", qemu_command.Gdb)

	parser.AddCommand("qemu",
		"Run benchmark in qemu",
		"Long description",
		&qemu_command)
}
