import parseopt
import strformat
import strutils
import osproc
import os
import times
import streams
import posix
from system import quit

type
  Time = tuple[hour:HourRange, min:MinuteRange, seconds:SecondRange]

proc log(a: varargs[string]) =
  let ts = now()
  write(stderr, &"[timewindow] {ts} ")
  for s in items(a):
    write(stderr, s)
  write(stderr, "\n")

proc toSeconds(a:Time): int =
  ## Convert a time to seconds since midnight
  return (a.hour * 60 * 60) + (a.min * 60) + (a.seconds)

proc nowSeconds(): int =
  let n = utc(now())
  let nowtime:Time = (hour:n.hour.HourRange, min:n.minute.MinuteRange, seconds:n.second.SecondRange)
  return toSeconds(nowtime)

const SECONDS_IN_DAY = 24 * 60 * 60

proc secondsToNextEvent(start:int, stop:int): int =
  var n = nowSeconds()
  if start > stop:
    # ---|   |---  run-time spans midnight
    if n >= start:
      # ---|   |-X-
      return SECONDS_IN_DAY - n + stop + 1
    elif n < stop:
      # -X-|   |---
      return stop - n + 1
    else:
      # ---| X |---
      return start - n + 1
  else:
    #    |---|     run-time fits in a single day
    if start <= n and n < stop:
      #    |-X-|
      return stop - n + 1
    elif n >= stop:
      #    |---| X 
      return SECONDS_IN_DAY - n + start + 1
    else:
      #  X |---|
      return start - n + 1


proc inStartWindow(start:int, stop:int):bool =
  if start == stop:
    return true
  let n = nowSeconds()
  # log(&"now: {n} start: {start} stop: {stop}")
  if start > stop:
    # ---|   |--- run-time spans midnight
    if n >= start:
      return true
    elif n < stop:
      return true
    else:
      return false
  else:
    #    |---|    run-time fits in a single day
    return start <= n and n < stop

# proc watchForKill(p:Process) =
  
#   onSignal(posix.SIGINT):
#     log("SIGINT received")
#     p.kill
    
#   onSignal(posix.SIGTERM):
#     log("SIGTERM received")

proc executeInTimewindow(args:seq[string], start:Time, stop:Time, stdout:string, stderr:string) =
  var
    start_s = toSeconds(start)
    stop_s = toSeconds(stop)
    p: Process
    rc: int
    is_paused: bool
    line = ""
    p_stdout, p_stderr: Stream
    o_stdout: FileStream
    o_stderr: FileStream

  if stdout != "" and stdout == stderr:
    # stdout and stderr will use the same stream
    if not existsFile(stdout):
      writeFile(stdout, "")
    o_stdout = openFileStream(stdout, fmAppend)
    o_stderr = o_stdout
  else:
    # stdout and stderr will use different streams
    if stdout != "":
      if not existsFile(stdout):
        writeFile(stdout, "")
      o_stdout = openFileStream(stdout, fmAppend)
    else:
      o_stdout = newFileStream(system.stdout)
    
    if stderr != "":
      if not existsFile(stderr):
        writeFile(stderr, "")
      o_stderr = openFileStream(stderr, fmAppend)
    else:
      o_stderr = newFileStream(system.stderr)

  log(&"now: {utc(now())}")

  while true:
    if p == nil:
      # not started yet
      if inStartWindow(start_s, stop_s):
        log("starting")
        p = startProcess(command=args[0], args=args[1..^1], options={poUsePath})
        p_stdout = outputStream(p)
        p_stderr = errorStream(p)
      else:
        let wait = secondsToNextEvent(start_s, stop_s)
        log(&"waiting {wait}s to start")
        sleep(wait * 1000)
    else:
      # started
      if peekExitCode(p) == -1:
        # not yet finished
        if inStartWindow(start_s, stop_s):
          if is_paused:
            # resume it
            log("resuming")
            resume(p)
            is_paused = false
          else:
            # keep running
            # log("running")
            # XXX I feel like the p_stderr buffer could get filled up, no?
            if readLine(p_stdout, line):
              writeLine(o_stdout, line)
              flush(o_stdout)
            elif readLine(p_stderr, line):
              writeLine(o_stderr, line)
              flush(o_stderr)
            else:
              # XXX it would be nice to use a less polling-like method
              sleep(1000)
            
        else:
          # pause it
          let wait = secondsToNextEvent(start_s, stop_s)
          log(&"pausing for {wait}s")
          suspend(p)
          is_paused = true
          sleep(wait * 1000)
      else:
        # finished
        while readLine(p_stdout, line):
          writeLine(o_stdout, line)
        while readLine(p_stderr, line):
          writeLine(o_stderr, line)
        rc = waitForExit(p)
        log(&"finished ({rc})")
        break

  quit(rc)

proc parseTime(x:string):Time =
  if x == "":
    return (hour:0.HourRange, min:0.MinuteRange, seconds:0.SecondRange)
  let parts = split(x, ":")
  return (hour:parseInt(parts[0]).HourRange, min:parseInt(parts[1]).MinuteRange, seconds:0.SecondRange)

proc writeHelp() =
  echo """
Usage:

    timewindow [opts] [--] COMMAND [ARG...]

Opts:

    --start-time=   HH:MM when this program should be running
    --stop-time=    HH:MM when this program should not be running
    --stdout=       File to append subcommand's stdout to.  If not
                    given, then write to this process' stdout.
    --stderr=       file to append subcommand's stderr to.  If not
                    given, then write to this process' stderr.
    -v              verbose output
"""

proc main() =
  var p = initOptParser()
  var
    started_cmd = false
    verbose = false
    stop_time, start_time: Time
    got_times = 0
    stdout, stderr = ""
    subargs: seq[string]

  for kind, key, val in p.getopt():
    case kind
    of cmdArgument:
      started_cmd = true
      subargs.add(key)
    of cmdLongOption:
      if started_cmd:
        if val != "":
          subargs.add(&"--{key}={val}")
        else:
          subargs.add(&"--{key}")
      else:
        case key
          of "start-time":
            start_time = parseTime(val)
            got_times = got_times + 1
          of "stop-time":
            stop_time = parseTime(val)
            got_times = got_times + 1
          of "stdout": stdout = val
          of "stderr": stderr = val
          of "": started_cmd = true
          of "help", "h": writeHelp()
          else:
            echo &"Error: unknown flag --{key}"
            quit(1)
    of cmdShortOption:
      if started_cmd:
        if val != "":
          subargs.add(&"-{key}={val}")
        else:
          subargs.add(&"-{key}")
      else:
        if key == "v":
          verbose = true
        elif key == "-":
          echo "--?"
        else:
          echo &"Error: unknown flag -{key}"
          quit(1)
    of cmdEnd: assert(false)
  
  if got_times == 1:
    echo &"Error: provide both --start-time and --stop-time or neither"
    quit(1)
  if verbose:
    echo "subargs ", subargs
    echo "start-time ", start_time
    echo "stop-time ", stop_time
  executeInTimewindow(subargs, start_time, stop_time, stdout, stderr)

if isMainModule:
  main()
