#!/bin/bash

RUNTIME=$1
EXITCODE=${2:-0}

# for testing
echo "samplechild args $@"
echo "samplechild start"
echo "samplechild pid $$"
echo "samplechild FOO=${FOO}"
echo "samplechild stdout"
echo "samplechild stderr" >&2


trap 'echo "samplechild got SIGINT"; exit -1' SIGINT
trap 'echo "samplechild got SIGTERM"; exit -2' SIGTERM

START=$(date +%s)
i=0
max="${RUNTIME}"
while [ "$i" -lt "$max" ]; do
    sleep 1
    if [ $(expr $i % 10) -eq 0 ]; then
        echo "samplechild has run for ${i}s"    
    fi
    true $(( i++ ))
done
STOP=$(date +%s)

echo "samplechild done"
TOOK=$(expr $STOP - $START)
echo "samplechild took ${TOOK}"
exit "$EXITCODE"
