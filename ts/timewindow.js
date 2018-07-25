#!/usr/bin/env node
(function (factory) {
    if (typeof module === "object" && typeof module.exports === "object") {
        var v = factory(require, exports);
        if (v !== undefined) module.exports = v;
    }
    else if (typeof define === "function" && define.amd) {
        define(["require", "exports", "child_process", "fs"], factory);
    }
})(function (require, exports) {
    "use strict";
    Object.defineProperty(exports, "__esModule", { value: true });
    const child_process_1 = require("child_process");
    const fs = require("fs");
    function log(...args) {
        console.error(`[timewindow] ${(new Date()).toISOString()}`, ...args);
    }
    function hhmm2seconds(x) {
        const parts = x.split(':');
        return parseInt(parts[0]) * 60 * 60 + parseInt(parts[1]) * 60;
    }
    function processArgs(args) {
        let subcommand = [];
        let ret = { subcommand };
        let in_subcommand = false;
        for (let arg of args) {
            if (in_subcommand) {
                subcommand.push(arg);
            }
            else {
                if (arg.startsWith('--start-time')) {
                    ret.start_time = hhmm2seconds(arg.split('=')[1]);
                }
                else if (arg.startsWith('--stop-time')) {
                    ret.stop_time = hhmm2seconds(arg.split('=')[1]);
                }
                else if (arg.startsWith('--stdout')) {
                    ret.stdout = arg.split('=')[1];
                }
                else if (arg.startsWith('--stderr')) {
                    ret.stderr = arg.split('=')[1];
                }
                else if (arg === '--') {
                    in_subcommand = true;
                }
                else {
                    in_subcommand = true;
                    subcommand.push(arg);
                }
            }
        }
        return ret;
    }
    function nowSeconds() {
        const d = new Date();
        return d.getUTCHours() * 60 * 60 + d.getUTCMinutes() * 60 + d.getUTCSeconds();
    }
    function inStartWindow(start, stop) {
        if (start === undefined && stop === undefined) {
            return true;
        }
        const now = nowSeconds();
        if (start > stop) {
            // ---|   |---  run-time spans midnight
            if (now >= start) {
                // ---|   |-X-
                return true;
            }
            else if (now < stop) {
                // -X-|   |---
                return true;
            }
            else {
                // ---| X |---
                return false;
            }
        }
        else {
            return start <= now && now < stop;
        }
    }
    const SECONDS_IN_DAY = 24 * 60 * 60;
    function secondsToNextEvent(start, stop) {
        if (start === undefined && stop === undefined) {
            return 1;
        }
        const now = nowSeconds();
        // log('now', now, 'start', start, 'stop', stop)
        if (start > stop) {
            // ---|   |---  run-time spans midnight
            if (now >= start) {
                // ---|   |-X-
                return SECONDS_IN_DAY - now + stop;
            }
            else if (now < stop) {
                // -X-|   |---
                return stop - now;
            }
            else {
                // ---| X |---
                return start - now;
            }
        }
        else {
            //    |---|     run-time fits in a single day
            if (start <= now && now < stop) {
                //    |-X-|
                return stop - now;
            }
            else if (now >= stop) {
                //    |---| X 
                return SECONDS_IN_DAY - now + start;
            }
            else {
                //  X |---|
                return start - now;
            }
        }
    }
    async function executeInTimeWindow(args) {
        return new Promise((resolve, reject) => {
            const start = args.start_time;
            const stop = args.stop_time;
            let stdout = process.stdout, stderr = process.stderr;
            if (args.stdout && args.stdout === args.stderr) {
                // log stdout and stderr to the same file
                stdout = stderr = fs.createWriteStream(args.stdout, { flags: 'a+' });
            }
            else {
                if (args.stdout) {
                    stdout = fs.createWriteStream(args.stdout, { flags: 'a+' });
                }
                if (args.stderr) {
                    stderr = fs.createWriteStream(args.stderr, { flags: 'a+' });
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
                }, 1000);
            });
            process.on('SIGTERM', () => {
                if (p) {
                    p.kill('SIGTERM');
                }
                setTimeout(() => {
                    process.exit(1);
                }, 1000);
            });
            function tick() {
                log('tick');
                if (!p) {
                    // not started yet
                    if (inStartWindow(start, stop)) {
                        log('starting');
                        p = child_process_1.spawn(args.subcommand[0], args.subcommand.slice(1), {});
                        p.stdout.on('data', data => {
                            stdout.write(data);
                        });
                        p.stderr.on('data', data => {
                            stderr.write(data);
                        });
                        p.on('close', code => {
                            rc = code;
                            if (timeout) {
                                clearTimeout(timeout);
                            }
                            resolve(code);
                        });
                        p.on('error', err => {
                            reject(err);
                        });
                    }
                    else {
                        // waiting to start
                    }
                }
                else {
                    // started and not yet finished
                    if (inStartWindow(start, stop)) {
                        if (is_paused) {
                            // resume it
                            log('resuming (SIGCONT)');
                            p.kill('SIGCONT');
                            is_paused = false;
                        }
                        else {
                            // keep running
                        }
                    }
                    else {
                        // pause it
                        log('pausing (SIGSTOP)');
                        p.kill('SIGSTOP');
                        is_paused = true;
                    }
                }
                if (rc === null) {
                    const wait = secondsToNextEvent(start, stop);
                    log(`waiting ${wait}s`);
                    timeout = setTimeout(tick, wait * 1000);
                }
            }
            tick();
        });
    }
    function main(args) {
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
});
