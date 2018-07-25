#!/usr/bin/env node

import { spawn } from 'child_process'
import * as fs from 'fs'

function log(...args) {
  console.error(`[timewindow] ${(new Date()).toISOString()}`, ...args);
}

function hhmm2seconds(x:string):number {
  const parts = x.split(':');
  return parseInt(parts[0]) * 60 * 60 + parseInt(parts[1]) * 60;
}

interface ICallArgs {
  start_time?: number;
  stop_time?: number;
  stdout?: string;
  stderr?: string;
  subcommand: string[];
}

function processArgs(args:string[]):ICallArgs {
  let subcommand:string[] = [];
  let ret:ICallArgs = {subcommand};
  let in_subcommand = false;
  for (let arg of args) {
    if (in_subcommand) {
      subcommand.push(arg)
    } else {
      if (arg.startsWith('--start-time')) {
        ret.start_time = hhmm2seconds(arg.split('=')[1])
      } else if (arg.startsWith('--stop-time')) {
        ret.stop_time = hhmm2seconds(arg.split('=')[1])
      } else if (arg.startsWith('--stdout')) {
        ret.stdout = arg.split('=')[1]
      } else if (arg.startsWith('--stderr')) {
        ret.stderr = arg.split('=')[1]
      } else if (arg === '--') {
        in_subcommand = true
      } else {
        in_subcommand = true
        subcommand.push(arg)
      }
    }
  }
  return ret
}

function nowSeconds():number {
  const d = new Date();
  return d.getUTCHours() * 60 * 60 + d.getUTCMinutes() * 60 + d.getUTCSeconds();
}

function inStartWindow(start:number, stop:number):boolean {
  if (start === undefined && stop === undefined) {
    return true
  }
  const now = nowSeconds()
  if (start > stop) {
    // ---|   |---  run-time spans midnight
    if (now >= start) {
      // ---|   |-X-
      return true
    } else if (now < stop) {
      // -X-|   |---
      return true
    } else {
      // ---| X |---
      return false
    }
  } else {
    return start <= now && now < stop
  }
}

const SECONDS_IN_DAY = 24 * 60 * 60

function secondsToNextEvent(start:number, stop:number) {
  if (start === undefined && stop === undefined) {
    return 1 
  }
  const now = nowSeconds();
  // log('now', now, 'start', start, 'stop', stop)
  if (start > stop) {
    // ---|   |---  run-time spans midnight
    if (now >= start) {
      // ---|   |-X-
      return SECONDS_IN_DAY - now + stop
    } else if (now < stop) {
      // -X-|   |---
      return stop - now
    } else {
      // ---| X |---
      return start - now
    }
  } else {
    //    |---|     run-time fits in a single day
    if (start <= now && now < stop) {
      //    |-X-|
      return stop - now
    } else if (now >= stop) {
      //    |---| X 
      return SECONDS_IN_DAY - now + start
    } else {
      //  X |---|
      return start - now
    }
  }
}

async function executeInTimeWindow(args:ICallArgs):Promise<number> {
  return new Promise<number>((resolve, reject) => {
    const start = args.start_time;
    const stop = args.stop_time;
    let stdout:any = process.stdout,
        stderr:any = process.stderr;
    if (args.stdout && args.stdout === args.stderr) {
      // log stdout and stderr to the same file
      stdout = stderr = fs.createWriteStream(args.stdout, {flags:'a+'})
    } else {
      if (args.stdout) {
        stdout = fs.createWriteStream(args.stdout, {flags:'a+'})
      }
      if (args.stderr) {
        stderr = fs.createWriteStream(args.stderr, {flags:'a+'})
      }
    }

    let p;
    let timeout;
    let rc = null;
    let is_paused = false;

    process.on('SIGINT', () => {
      if (p) {
        p.kill('SIGINT');
      }
      setTimeout(() => {
        process.exit(1);  
      }, 1000)
    })
    process.on('SIGTERM', () => {
      if (p) {
        p.kill('SIGTERM');
      }
      setTimeout(() => {
        process.exit(1);  
      }, 1000)
    })

    function tick() {
      log('tick');
      if (!p) {
        // not started yet
        if (inStartWindow(start, stop)) {
          log('starting');
          p = spawn(args.subcommand[0], args.subcommand.slice(1), {
          })
          p.stdout.on('data', data => {
            stdout.write(data);
          })
          p.stderr.on('data', data => {
            stderr.write(data);
          })
          p.on('close', code => {
            rc = code;
            if (timeout) {
              clearTimeout(timeout);
            }
            resolve(code);
          })
          p.on('error', err => {
            reject(err);
          })
        } else {
          // waiting to start
        }
      } else {
        // started and not yet finished
        if (inStartWindow(start, stop)) {
          if (is_paused) {
            // resume it
            log('resuming (SIGCONT)')
            p.kill('SIGCONT');
            is_paused = false;
          } else {
            // keep running
          }
        } else {
          // pause it
          log('pausing (SIGSTOP)')
          p.kill('SIGSTOP')
          is_paused = true
        }
      }
      if (rc === null) {
        const wait = secondsToNextEvent(start, stop);
        log(`waiting ${wait}s`);
        timeout = setTimeout(tick, wait * 1000);  
      }
    }
    tick();
  })
}

function main(args:string[]) {
  let callargs = processArgs(args);
  if ((callargs.start_time !== undefined && callargs.stop_time === undefined) || (callargs.start_time === undefined && callargs.stop_time !== undefined)) {
    console.error('Provide both --start-time and --stop-time or neither');
    process.exit(1);
  }
  log(callargs);

  return executeInTimeWindow(callargs);
}

if (require.main === module) {
  main(process.argv.slice(2))
  .then(code => {
    process.exit(code);
  })
  .catch(err => {
    console.log('Error:', err);
    process.exit(1);
  });
}
