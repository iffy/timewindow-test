package main

import (
	"fmt"
	//flag "github.com/spf13/pflag"
	"flag"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
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
		log.Println()
		log.Printf("Proxy signal %v to child.", sig)
		cmd.Process.Signal(sig)
	}()
}

func hhmm2Seconds(x string) int {
	parts := strings.Split(x, ":")
	h, err := strconv.Atoi(parts[0])
	if err != nil {
		log.Fatal(err)
	}
	m, err := strconv.Atoi(parts[1])
	if err != nil {
		log.Fatal(err)
	}
	return h * 60 * 60 + m * 60
}

func nowSeconds() int {
	now := time.Now().UTC()
	return now.Hour() * 60 * 60 + now.Minute() * 60 + now.Second()
}

func inStartWindow(start int, stop int) bool {
	// if (start == nil && stop == nil || start == stop) {
	// 	return true
	// }
	now := nowSeconds()
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

func secondsToNextEvent(start int, stop int) time.Duration {
	// if (start == nil && stop == nil) {
	// 	return 0
	// }
	return time.Duration(func() int {
		now := nowSeconds()
		if (now < start) {
			if (now < stop) {
				return stop - now
			} else {
				return start - now
			}
		} else if (now < stop) {
			return stop - now
		} else {
			SECONDS_IN_DAY := 24 * 60 * 60
			if (start < stop) {
				return SECONDS_IN_DAY - now + start
			} else {
				return SECONDS_IN_DAY - now + stop
			}
		}
	}()) * time.Second
}

func tick() {

}

func main() {
	logger := log.New(os.Stderr, "[timewindow] ", log.LstdFlags)
	flag.Usage = Usage
	flag.Parse()
	runme := flag.Args()
	logger.Printf("runme is %s\n", runme)

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

	logger.Println("Current UTC time", time.Now().UTC())

	start := hhmm2Seconds(*starttime)
	stop := hhmm2Seconds(*stoptime)

	done := make(chan error)
	var pause <-chan time.Time
	var resume <-chan time.Time

	start_after := 0 * time.Second
	if ! inStartWindow(start, stop) {
		start_after = secondsToNextEvent(start, stop)
		logger.Println("waiting to start", start_after)
	}
	do_start := time.After(start_after)

	for {
		select {
		case <-do_start:
			logger.Println("Start command")
 			cmd.Start()
 			proxySignals(cmd)
 			go func() { done <- cmd.Wait() }()
 			
 			time_to_pause := secondsToNextEvent(start, stop)
 			logger.Println("Will pause after", time_to_pause)
 			pause = time.After(time_to_pause)
		case <-pause:
			logger.Println("Pausing command")
			cmd.Process.Signal(syscall.SIGSTOP)

			time_to_resume := secondsToNextEvent(start, stop)
			logger.Println("Will resume after", time_to_resume)
			resume = time.After(time_to_resume)
		case <-resume:
			logger.Println("Resuming command")
			cmd.Process.Signal(syscall.SIGCONT)

			time_to_pause := secondsToNextEvent(start, stop)
 			logger.Println("Will pause after", time_to_pause)
 			pause = time.After(time_to_pause)
		case err := <-done:
			logger.Println("Command done")
			if exiterr, ok := err.(*exec.ExitError); ok {
				if status, ok := exiterr.Sys().(syscall.WaitStatus); ok {
					os.Exit(status.ExitStatus())
				} else {
					logger.Printf("ERROR: failed to get WaitStatus: %s\n", err)
				}
			} else {
				os.Exit(0)
			}
		}
	}
}
