package main

import (
	"fmt"
	//flag "github.com/spf13/pflag"
	"flag"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"
)

var (
	starttime      = flag.String("start-time", "", "HH:MM when this program should be running")
	stoptime       = flag.String("stop-time", "", "HH:MM when this program should not be running")
	stdoutfilename = flag.String("stdout", "", "File to append subcommand's stdout to.  If not given, then write to this process's stdout.")
	stderrfilename = flag.String("stderr", "", "file to append subcommand's stderr to.  If not given, then write to this process's stderr.")
)

func Usage() {
	fmt.Fprintf(os.Stderr, `Usage:

      timewindow [opts] [--] COMMAND [ARG...]

`)
	flag.PrintDefaults()
}

func proxySignals(cmd *exec.Cmd) {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM, syscall.SIGKILL)

	go func() {
		sig := <-sigs
		fmt.Println()
		fmt.Printf("Proxy signal %v to child.", sig)
		cmd.Process.Signal(sig)
	}()
}

func main() {
	flag.Usage = Usage
	flag.Parse()
	runme := flag.Args()
	fmt.Printf("runme is %s\n", runme)

	var cmd *exec.Cmd

	switch len(runme) {
	case 0:
		fmt.Println("Try --help")
		return
	case 1:
		cmd = exec.Command(runme[0])
	default:
		cmd = exec.Command(runme[0], runme[1:]...)
	}

	if *stdoutfilename != "" {
		fh, err := os.OpenFile(*stdoutfilename, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
		if err != nil {
			fmt.Printf("ERROR with file \"%s\": %s", *stdoutfilename, err)
			return
		}
		cmd.Stdout = fh
		defer fh.Close()
	} else {
		cmd.Stdout = os.Stdout
	}
	if *stderrfilename != "" {
		fh, err := os.OpenFile(*stderrfilename, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
		if err != nil {
			fmt.Printf("ERROR with file \"%s\": %s", *stderrfilename, err)
			return
		}
		cmd.Stderr = fh
		defer fh.Close()
	} else {
		cmd.Stderr = os.Stderr
	}
	if *starttime == *stoptime {
		err := cmd.Run()
		proxySignals(cmd)
		if exiterr, ok := err.(*exec.ExitError); ok {
			if status, ok := exiterr.Sys().(syscall.WaitStatus); ok {
				os.Exit(status.ExitStatus())
			}
		}
		return
	}
	if *starttime == "" {
		fmt.Println("Not running start-time is empty")
		os.Exit(1)
	}
	if *stoptime == "" {
		fmt.Println("Not running stop-time is empty")
		os.Exit(1)
	}

	hm, err := time.Parse("15:04", *starttime)
	if err != nil {
		fmt.Println("Could not parse start-time: %s", err)
	}
	hm2, err := time.Parse("15:04", *stoptime)
	if err != nil {
		fmt.Println("Could not parse stop-time: %s", err)
	}

	now := time.Now()
	start := time.Date(now.Year(), now.Month(), now.Day(), hm.Hour(), hm.Minute(), now.Second(), 0, time.UTC)
	stop := time.Date(now.Year(), now.Month(), now.Day(), hm2.Hour(), hm2.Minute(), now.Second(), 0, time.UTC)

	var done chan error
	for {
		if stop.UnixNano() < start.UnixNano() {
			fmt.Println("Spans midnight")
			stop = stop.Add(time.Hour * 24)
		}
		if now.UnixNano() >= start.UnixNano() {
			if now.UnixNano() > stop.UnixNano() {
				fmt.Println("Time has passed already, rescheduling.")
				start = start.Add(time.Hour * 24)
				stop = stop.Add(time.Hour * 24)
				now = time.Now()
			} else {
				// start command
				if cmd.Process != nil {
					fmt.Println("This process was already started but didn't finish yet.")
					cmd.Process.Signal(syscall.SIGCONT)
				} else {
					fmt.Println("Start command")
					cmd.Start()
					proxySignals(cmd)
					done = make(chan error)
					go func() { done <- cmd.Wait() }()
				}
				runfor := stop.Sub(now)
				fmt.Printf("running for %s\n", runfor)
				timeout := time.After(runfor)
				// schedule kill for later
				select {
				case <-timeout:
					fmt.Println("SIGSTOP")
					cmd.Process.Signal(syscall.SIGSTOP)
					start = start.Add(time.Hour * 24)
					stop = stop.Add(time.Hour * 24)
					now = time.Now()
				case err := <-done:
					if exiterr, ok := err.(*exec.ExitError); ok {
						if status, ok := exiterr.Sys().(syscall.WaitStatus); ok {
							os.Exit(status.ExitStatus())
						} else {
							fmt.Printf("ERROR: failed to get WaitStatus: %s\n", err)
						}
					} else {
						os.Exit(0)
					}
				}
			}
		} else {
			// wait to start
			// now < start
			sleepfor := start.Sub(now)
			fmt.Printf("Start in %s\n", sleepfor)
			time.Sleep(sleepfor)
			now = time.Now()
		}

	}
}
