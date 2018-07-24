#!/bin/sh

RUNTIME=$1
EXITCODE=${2:-0}

i=0
max="${RUNTIME}"
while [ $i -lt $max ]; do
    sleep 1
    if [ $(expr $i % 10) -eq 0 ]; then
        echo "Runtime ${i}s"    
    fi
    true $(( i++ ))
done

exit "$EXITCODE"
