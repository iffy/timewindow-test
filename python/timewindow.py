#!/usr/bin/env python
from __future__ import print_function
import subprocess
import click
import signal
import sys
import time
from datetime import datetime

def log(*x):
    prefix = '[timewindow] {0} '.format(datetime.now().isoformat())
    sys.stderr.write(prefix + ' '.join(map(str, x)) + '\n')
    sys.stderr.flush()

def nowTime():
    return parseTime(datetime.utcnow().strftime('%H:%M'))

def parseTime(x):
    h, m = x.split(':')
    h = int(h, 10)
    m = int(m, 10)
    return (h,m)

SECONDS_IN_DAY = 24 * 60 * 60

def timeToSeconds((h,m)):
    return (m * 60) + (h * 60 * 60)


def inStartWindow(start, stop):
    if not start and not stop:
        return True
    now = nowTime()
    if start > stop:
        # ---|   |---  run-time spans midnight
        if now >= start:
            # ---|   |-X-
            return True
        elif now < stop:
            # -X-|   |---
            return True
        else:
            # ---| X |---
            return False
    else:
        #    |---|     run-time fits in a single day
        return start <= now < stop

def secondsToNextEvent(start, stop):
    if not start and not stop:
        return 1
    now = nowTime()
    now_seconds = timeToSeconds(now) + int(datetime.utcnow().strftime('%S'))
    print('now', now, 'start', start, 'stop', stop)
    if start > stop:
        # ---|   |---  run-time spans midnight
        if now >= start:
            # ---|   |-X-
            return SECONDS_IN_DAY - now_seconds + timeToSeconds(stop) + 1
        elif now < stop:
            # -X-|   |---
            return timeToSeconds(stop) - now_seconds + 1
        else:
            # ---| X |---
            return timeToSeconds(start) - now_seconds + 1
    else:
        #    |---|     run-time fits in a single day
        if start <= now < stop:
            #    |-X-|
            return timeToSeconds(stop) - now_seconds + 1
        elif now >= stop:
            #    |---| X 
            return SECONDS_IN_DAY - now_seconds + timeToSeconds(start) + 1
        else:
            #  X |---|
            return timeToSeconds(start) - now_seconds + 1

def executeInTimewindow(args, start, stop, stdout, stderr):
    p = None
    if stdout and stdout == stderr:
        stdout = stderr = open(stdout, 'a+')
    else:
        if stdout:
            stdout = open(stdout, 'a+')
        if stderr:
            stderr = open(stderr, 'a+')
    is_paused = False
    while True:
        if p is None:
            # not started yet
            if inStartWindow(start, stop):
                log('starting')
                p = subprocess.Popen(args, stdout=stdout, stderr=stderr, stdin=subprocess.PIPE)
                # p.stdin.close()
            else:
                wait = secondsToNextEvent(start, stop)
                log('waiting {0}s to start'.format(wait))
                time.sleep(wait)
        else:
            # started
            if p.poll() is None:
                # not yet finished
                if inStartWindow(start, stop):
                    if is_paused:
                        # resume it
                        log('resuming (SIGCONT)')
                        p.send_signal(signal.SIGCONT)
                        is_paused = False
                    else:
                        # keep running
                        # XXX it would be nice to use something like select here instead of polling.
                        time.sleep(1)
                else:
                    # pause it
                    wait = secondsToNextEvent(start, stop)
                    log('pausing (SIGSTOP) for {0}s'.format(wait))
                    p.send_signal(signal.SIGSTOP)
                    is_paused = True
                    time.sleep(wait)
            else:
                # finished
                log('finished ({0})'.format(p.returncode))
                sys.exit(p.returncode)

def optionalTime(x):
    if not x:
        return None
    else:
        return parseTime(x)

@click.command()
@click.option('--start-time', type=optionalTime)
@click.option('--stop-time', type=optionalTime)
@click.option('--stdout')
@click.option('--stderr')
@click.argument('args', nargs=-1, required=True)
def main(start_time, stop_time, stdout, stderr, args):
    if start_time == stop_time:
        start_time = None
        stop_time = None
    if (start_time or stop_time) and not (start_time and stop_time):
        raise Exception('You must either specify both start-time and stop-time or use neither.')
    executeInTimewindow(args,
        start=start_time,
        stop=stop_time,
        stdout=stdout,
        stderr=stderr)

if __name__ == '__main__':
    main()
