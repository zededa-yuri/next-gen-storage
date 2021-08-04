import click
from netmiko import ConnectHandler
import os

@click.group()
def cli():
    pass

@cli.command()
@click.option('--target', help='Target host to run', required=True)
def bench(target):
    key_file = os.path.expanduser("~/.ssh/id_rsa")
    host = {
        'device_type': 'generic',
        'host':   target,
        'username': 'yuri',
        "use_keys": True,
        "key_file": key_file,
    }

    host_connection = ConnectHandler(**host)
    output = host_connection.send_command('uname')
    print(output)
