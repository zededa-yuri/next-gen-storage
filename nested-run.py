#!/usr/bin/env python3
import subprocess
import os
import random

def main():
    my_pid = os.getpid()
    print(f"my pid is {my_pid}")

    script_path = os.path.realpath(__file__)
    script_path = os.path.dirname(script_path)

    qemu_cmd = [
        f"{script_path}/qemu/build/qemu-system-x86_64",
         "-nographic",
        "-enable-kvm", "-m", "1024",
        '-cpu', 'host',
        '-smp', '2',
        "-device", "e1000,netdev=net0",
        "-netdev", "user,id=net0,hostfwd=tcp::5551-:22",
        "-append", "console=ttyS0 root=/dev/vda3",
        "-kernel", f"{script_path}/linux/arch/x86_64/boot/bzImage",
        "-drive", f"file={script_path}/alpine.qcow2,if=virtio",
        "-readconfig", f"{script_path}/configs/nvme-vhost.cfg",
    ]

    print("launching:\n" + " ".join(qemu_cmd))
    p = subprocess.Popen(qemu_cmd)
    p.wait()
    print("~~~ Goodbye ~~~")
    
if __name__ == "__main__":
    main()
