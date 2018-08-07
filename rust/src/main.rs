#[macro_use]
extern crate log;
extern crate stderrlog;
extern crate clap;
extern crate subprocess;
use clap::{Arg, App};
use std::process::{Command};
use std::fs::OpenOptions;
use subprocess::{Popen, PopenConfig, Redirection};

fn main() {
    stderrlog::new()
        .module(module_path!())
        .verbosity(5)
        .timestamp(stderrlog::Timestamp::Second)
        .init()
        .unwrap();

    let matches = App::new("timewindow")
        .version("0.1.0")
        .about("Runs a command during a timewindow and pauses it outside the window")
        .arg(Arg::with_name("start-time")
            .long("start-time")
            .help("HH:MM when this program should be running")
            .takes_value(true))
        .arg(Arg::with_name("stop-time")
            .long("stop-time")
            .help("HH:MM when this program should not be running")
            .takes_value(true))
        .arg(Arg::with_name("stdout")
            .help("File to append subcommand's stdout to.  If not given, then write to this process' stdout.")
            .long("stdout")
            .takes_value(true))
        .arg(Arg::with_name("stderr")
            .long("stderr")
            .help("File to append subcommand's stderr to.  If not given, then write to this process' stderr.")
            .takes_value(true))
        .arg(Arg::with_name("command")
            .value_name("COMMAND")
            .required(true)
            .takes_value(true))
        .arg(Arg::with_name("args")
            .multiple(true)
            .value_name("ARGS")
            .help("Additional arguments to COMMAND")
            .takes_value(true))
        .get_matches();

    let start_time = matches.value_of("start-time").unwrap_or("");
    let stop_time = matches.value_of("stop-time").unwrap_or("");
    if (start_time.is_empty() && !stop_time.is_empty()) || (stop_time.is_empty() && !start_time.is_empty()) {
        error!("Either provide both --start-time and --stop-time or neither");
        ::std::process::exit(1);
    }
    let stdout = matches.value_of("stdout").unwrap_or("");
    let stderr = matches.value_of("stderr").unwrap_or("");

    let command = matches.value_of("command").unwrap();
    let mut args = matches.values_of("args").unwrap().collect::<Vec<_>>();
    info!("command: {} {:?}", command, args);

    if !start_time.is_empty() {
        // Using a timewindow
        info!("Using a timewindow");
        
        let mut cmd_and_args = vec![command];
        cmd_and_args.append(&mut args);
        let mut p = Popen::create(&cmd_and_args, PopenConfig {
            stdout: Redirection::Pipe, ..Default::default()

        }).unwrap();
        let (out, err) = p.communicate(None).unwrap();

        info!("out {:?}", out);
        info!("err {:?}", err);
        if let Some(exit_status) = p.poll() {
            info!("finished {:?}", exit_status);
        } else {
            info!("still running");
            p.terminate().unwrap();
        }
    } else {
        // Not using a timewindow
        info!("Not using a timewindow");
        let mut child = Command::new(command);
        child.args(args);
        if !stdout.is_empty() {
            let file = OpenOptions::new().append(true).create(true).open(stdout).unwrap();
            child.stdout(file);
        }
        if !stderr.is_empty() {
            let file = OpenOptions::new().append(true).create(true).open(stderr).unwrap();
            child.stderr(file);
        }
        let mut spawned = child
            .spawn()
            .expect("Failed to run?");
        let ecode = spawned.wait()
            .expect("failed to wait on child");
        info!("Done {}", ecode);
        match ecode.code() {
            Some(code) => ::std::process::exit(code),
            None       => ::std::process::exit(1)
        }
    }
}
