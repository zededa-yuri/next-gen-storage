package main

import (
	"bytes"
	"fmt"
	"time"
	"os"
	"os/exec"
	"context"
	"text/template"
)


type QemuCommand struct {
	Gdb bool `short:"g" long:"gdb" description:"just at test argurment"`
	Verbose []bool `short:"v" long:"verbose" description:"Show verbose debug information"`
}

var qemu_command QemuCommand

type mainTemplateArgs struct {
	foo string
}

func write_main_config(path string, template_args mainTemplateArgs) error {
	t, err := template.New("qemu").Parse(qemuConfTemplate)

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

func qemu_run(ctx context.Context, cancel context.CancelFunc) {

	template_args := mainTemplateArgs{"bar"}

	qemu_main_config_path := "qemu.cfg"
	err := write_main_config(qemu_main_config_path, template_args)
	if err != nil {
		return
	}

	cmd := exec.CommandContext(ctx,
		"qemu-system-x86_64",
		"-readconfig",  qemu_main_config_path,
		"-display", "none",
		"-device", "e1000,netdev=net0",  "-netdev",  "user,id=net0,hostfwd=tcp::6666-:22",
		"-serial", "chardev:ch0")

	// cmd := exec.CommandContext(ctx, "ls", "-z")

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	fmt.Printf("Running a command\n")
	err = cmd.Run()

	if ctx.Err() == context.Canceled || ctx.Err() == context.DeadlineExceeded {
		fmt.Printf("Cancelled: %v\n", ctx.Err())
		return
	} else if err != nil {
		fmt.Printf("error launching command: %v; err=%v\n", err, ctx.Err())
		fmt.Printf("%s\n", out)
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
