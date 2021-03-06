import click
from netmiko import ConnectHandler
from netmiko import file_transfer
import os,sys
import configparser
import logging
import shutil
import datetime
import subprocess
import gevent
from pssh.clients import SSHClient


logger = logging.getLogger('autobench')
logger.setLevel(logging.DEBUG)

ch = logging.StreamHandler()
ch.setLevel(logging.DEBUG)
formatter = logging.Formatter('%(asctime)s - %(name)s - %(levelname)s - %(message)s')
ch.setFormatter(formatter)
logger.addHandler(ch)

@click.group()
def cli():
    pass


class bench_set:
    def __init__(self, out_dir, target):
        self.out_dir = out_dir
        self.host = SSHClient(target)
        self.target_home = ''.join(self.host.run_command('pwd').stdout)
        logger.debug(f'target home is {self.target_home}')

    def gen_qemu_config(self):
        config = configparser.ConfigParser()
        config['machine'] = {
            'type'            : '"pc-q35-3.1"',
            'dump-guest-core' : '"off"',
            'accel'           : '"kvm"',
            'vmport'          : '"off"',
            'kernel-irqchip'  : '"on"',
            'graphics'        : '"off"',
        }

        config['memory'] = {
            'size' : '"8192"',
        }

        self.qemu_config = 'qemu.cfg'
        with open(os.path.join(self.out_dir, self.qemu_config), 'w') as f:
            config.write(f)
    
    def start_qemu(self):
        self.gen_qemu_config()

        logger.debug("uploading /tmp/qemu.cfg")
        cmds = self.host.copy_file(os.path.join(self.out_dir, self.qemu_config),
                                   '/tmp/qemu.cfg')

                                                         
        qemu_bin = os.path.join(self.target_home, 'qemu/build/qemu-system-x86_64')
        qemu_cmd = [
            qemu_bin,
            "-cpu", "host",
            "-smp", "6",

            '-display', 'none',
            '-chardev', 'stdio,mux=on,id=ch0',
            '-mon', 'chardev=ch0,mode=readline',

            "-device", "e1000,netdev=net0", "-netdev",
            
            "user,id=net0" +
            ",hostfwd=tcp::5555-:22",

            '-drive', f'file=bionic-server-cloudimg-i386.img,if=virtio',
            "-readconfig", '/tmp/qemu.cfg',
        ]

        qemu_cmd_str = subprocess.list2cmdline(qemu_cmd)
        logger.debug(f'launching {qemu_cmd_str}')
        qemu_proc = self.host.run_command(qemu_cmd_str)
        self.host.wait_finished(qemu_proc)
        print(qemu_proc)
        print('\n'.join(qemu_proc.stdout))


    def create_work_dir(self):
        self.start_time = datetime.datetime.utcnow()
        target_out_dir = os.path.join(self.target_home, f'bench-{self.start_time.isoformat()}')
        logger.debug(f'creating target dir {target_out_dir}')
        self.host.run_command(f'mkdir {target_out_dir}')
        self.target_out_dir = target_out_dir
    
@cli.command()
@click.option('--target', help='Target host to run', required=True)
@click.option('--output', help='Directory to store artefacts', default='.out')
def bench(output, target):
    if os.path.exists(output):
        if output == '.out':
            shutil.rmtree('.out')
        else:
            logger.error(f'directory {output} exists')
            sys.exit(1)

    os.mkdir(output)
    bench = bench_set(output, target)
    bench.create_work_dir()
    bench.start_qemu()

