package main

import (
	"bytes"
	"fmt"
	"time"
	"os/exec"
	"context"
)

type QemuCommand struct {
	Gdb bool `short:"g" long:"gdb" description:"just at test argurment"`
	Verbose []bool `short:"v" long:"verbose" description:"Show verbose debug information"`
}

var qemu_command QemuCommand

func qemu_run(ctx context.Context, cancel context.CancelFunc) {
	cmd := exec.CommandContext(ctx, "sleep", "6")
	var out bytes.Buffer
	cmd.Stdout = &out

	fmt.Printf("Running a command\n")
	err := cmd.Run()

	if ctx.Err() == context.Canceled || ctx.Err() == context.DeadlineExceeded {
		fmt.Printf("Cancelled: %v\n", ctx.Err())
		return
	} else if err != nil {
		fmt.Printf("error launching command: %v; err=%v\n", err, ctx.Err())
		cancel()
	} else {
		fmt.Printf("command returned:\n%s\n", out)
	}

}

func (x *QemuCommand) Execute(args []string) error {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 5 * time.Second)
	defer cancel()

	qemu_run(ctx, cancel)
	
	return nil
}

func init() {
	fmt.Printf("--- %v\n", qemu_command.Gdb)

	parser.AddCommand("qemu",
		"Run benchmark in qemu",
		"Long description",
		&qemu_command)
}
