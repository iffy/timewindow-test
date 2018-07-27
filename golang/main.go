package main

import (
	"flag"
	"fmt"
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

type runner struct {
	*exec.Cmd
}

func (cmd *runner) start() {

	// start command
	if cmd.Process != nil {
		fmt.Println("This process was already started but didn't finish yet.")
		cmd.Process.Signal(syscall.SIGCONT)
	} else {
		fmt.Println("Start command")
		proxySignals(cmd)
		cmd.Start()
		go func() {
			err := cmd.Wait()
			if exiterr, ok := err.(*exec.ExitError); ok {
				if status, ok := exiterr.Sys().(syscall.WaitStatus); ok {
					os.Exit(status.ExitStatus())
				} else {
					fmt.Printf("ERROR: failed to get WaitStatus: %s\n", err)
					os.Exit(0)
				}
			} else {
				os.Exit(0)
			}
		}()
	}

}

func proxySignals(cmd *runner) {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM, syscall.SIGKILL)

	go func() {
		sig := <-sigs
		fmt.Printf("Proxy signal %v to child.", sig)
		cmd.Process.Signal(sig)
	}()
}

func secondsFromMidnight(now time.Time) int32 {
	return int32(now.Hour()*60*60 + now.Minute()*60 + now.Second())
}

func main() {
	flag.Usage = Usage
	flag.Parse()
	runme := flag.Args()
	fmt.Printf("runme is %s\n", runme)

	var cmd = new(runner)

	switch len(runme) {
	case 0:
		fmt.Println("Try --help")
		return
	case 1:
		cmd.Cmd = exec.Command(runme[0])
	default:
		cmd.Cmd = exec.Command(runme[0], runme[1:]...)
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
		proxySignals(cmd)
		cmd.start()
		select {} // TODO or not TODO that is the question.
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

	var midnightsec = int32(24 * 60 * 60)

	for {
		now := time.Now().In(time.UTC)
		start := time.Date(now.Year(), now.Month(), now.Day(), hm.Hour(), hm.Minute(), now.Second(), 0, time.UTC)
		stop := time.Date(now.Year(), now.Month(), now.Day(), hm2.Hour(), hm2.Minute(), now.Second(), 0, time.UTC)
		nowsec := secondsFromMidnight(now)
		stopsec := secondsFromMidnight(stop)
		startsec := secondsFromMidnight(start)
		fmt.Printf("Start: %v\nStop: %v\nNow: %v\n", start, stop, now)
		if nowsec < startsec {
			if nowsec >= stopsec || stopsec > startsec {
				// wait to start
				w := time.Second * time.Duration(startsec-nowsec)
				fmt.Printf("Starting in %v\n", w)
				time.Sleep(w)
			} else {
				cmd.start()
				r := time.Second * time.Duration(stopsec-nowsec)
				fmt.Printf("Will stop in %v\n", r)
				time.Sleep(r)
				cmd.Cmd.Process.Signal(syscall.SIGSTOP)
			}
		} else {
			if nowsec < stopsec || stopsec < startsec {
				// start
				cmd.start()
				var r time.Duration
				if stopsec < startsec {
					r = time.Second * time.Duration(midnightsec-nowsec)
					fmt.Printf("Running till midnight %v\n", r)

					time.Sleep(r)
				} else {
					r = time.Second * time.Duration(stopsec-nowsec)
					fmt.Printf("Running till stop time %v\n", r)
					time.Sleep(r)
					cmd.Cmd.Process.Signal(syscall.SIGSTOP)
				}
			} else {
				w := time.Second * time.Duration(midnightsec-stopsec)
				fmt.Printf("Waiting till midnight in the stop window %v\n", w)
				time.Sleep(w)
			}

		}
	}

}
