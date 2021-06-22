#!/usr/bin/env python3

import subprocess
import os
import argparse

def main():
    parser = argparse.ArgumentParser(description='Launch virtual hosts')
    parser.add_argument('--index', '-i',  default=1, type=int)

    args = parser.parse_args()

    if args.index > 9 or args.index < 1:
        parser.exit(status=1, message="index must be in range [1:9]\n")

    my_pid = os.getpid()
    print(f"my pid is {my_pid}")

    home=os.path.expanduser('~')
    cwd=os.getcwd()
    qemu_cmd = [
        "qemu-system-x86_64",
        "-nographic",
        "-cpu", "host",
        "-smp", "6",
        "-device", "e1000,netdev=net0", "-netdev", f"user,id=net0,hostfwd=tcp::555{args.index}-:22",
        '-drive', f'file=ubuntu-{args.index}.img,if=virtio',
        "-readconfig", "configs/virthost.cfg",
        "-kernel", "linux/arch/x86_64/boot/bzImage",
        "-append", "console=ttyS0 root=/dev/vda rw",
        "-gdb", f"tcp::{args.index}234",
        '-fsdev', f'local,id=src,path={home}/src,security_model=passthrough,readonly=off',
        '-device', 'virtio-9p-pci,fsdev=src,mount_tag=src',
    ]

    print('launching:\n' + ' '.join(qemu_cmd))
    p = subprocess.Popen(qemu_cmd)
    p.wait()
    print("~~~ Goodbye ~~~")

if __name__ == "__main__":
    main()
