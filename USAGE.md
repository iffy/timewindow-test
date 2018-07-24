Usage:

    timewindow [opts] COMMAND [ARG...]

Opts:

    --start-time=   HH:MM when this program should be running
    --stop-time=    HH:MM when this program should not be running
    --stdout=       File to append subcommand's stdout to.  If not
                    given, then write to this process' stdout.
    --stderr=       file to append subcommand's stderr to.  If not
                    given, then write to this process' stderr.

--start-time and --stop-time define a window during which the subcommand should be running.  Running this program will ensure that COMMAND [ARG...] is started during the timewindow and stopped when outside the timewindow.

--start-time and --stop-time are specified in 24-Hour format in the UTC timezone.

If neither --start-time nor --stop-time is given, COMMAND is run immediately and continues until completion.

If --start-time and --stop-time are the same, COMMAND is run immediately and continues until completion (as if neither option were given).

The program fails to run if only one of --start-time or --stop-time is given.

This program exits with the same return code that COMMAND returns as soon as COMMAND exits.

Environment variables provided to this command are provided to COMMAND.

When --stop-time is reached, the subprocess will be sent SIGSTOP (19).  If the process is stopped and --start-time is reached, the process will be sent SIGCONT (18).

All other signals sent to this program will be passed on to the child process.

