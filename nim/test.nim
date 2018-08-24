import posix
import osproc

proc main() =
    var
        p: Process

    proc doKill() =
        if p != nil:
            p.kill
    
    onSignal(posix.SIGINT):
        doKill()