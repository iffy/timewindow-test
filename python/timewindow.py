#!/usr/bin/env python
from __future__ import print_function
import subprocess
import click

@click.command()
@click.option('--start-time')
@click.option('--stop-time')
@click.option('--stdout')
@click.option('--stderr')
@click.argument('args', nargs=-1, required=True)
def main(start_time, stop_time, stdout, stderr, args):
    print('hey', args)

if __name__ == '__main__':
    main()
