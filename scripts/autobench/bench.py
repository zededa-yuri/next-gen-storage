import click

@click.group()
def cli():
    pass

@cli.command()
def bench():
    print("hello")

