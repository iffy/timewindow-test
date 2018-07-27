#!/bin/bash


SCRIPT=${1}
if [ -z "$SCRIPT" ]; then
    echo "Error: Usage $0 PROGRAM"
    exit 1
fi

FAILURES=""
[ -e _testfailures ] && rm _testfailures
addFailure() {
    echo "----------------------------"
    echo "FAILURE: $1"
    echo "----------------------------"
    echo " - $1" >> _testfailures
}

# return code 0
"$SCRIPT" ./samplechild.sh 0 0
RC=$?
[ "$RC" -eq "0" ] || addFailure "should have returned 0"

# return code 1
"$SCRIPT" ./samplechild.sh 0 1
RC=$?
[ "$RC" -eq "1" ] || addFailure "should have returned 1"

# stdout to file
echo "something" > _testoutput
"$SCRIPT" --stdout=_testoutput ./samplechild.sh 0 0 
grep "samplechild stdout" _testoutput || addFailure "--stdout"
grep "something" _testoutput|| addFailure "--stdout append"

# stderr to file
echo "something" > _testoutput
"$SCRIPT" --stderr=_testoutput ./samplechild.sh 0 0 
grep "samplechild stderr" _testoutput || addFailure "--stderr"
grep "something" _testoutput || addFailure "--stderr append"

# stdout and stderr to same file
[ -e _testoutput ] && rm _testoutput
"$SCRIPT" --stdout=_testoutput --stderr=_testoutput ./samplechild.sh 0 0 
grep "samplechild stderr" _testoutput || addFailure "stderr to same file as stdout"
grep "samplechild stdout" _testoutput || addFailure "stdout to same file as stderr"

# stdout to parent
stdout=$("$SCRIPT" ./samplechild.sh 0 0)
echo "$stdout" | grep "samplechild stdout" || addFailure "stdout is not sent through parent"

# stderr to parent
[ -e _testoutput ] && rm _testoutput
"$SCRIPT" ./samplechild.sh 0 0 >/dev/null 2>_testoutput
grep "samplechild stderr" _testoutput || addFailure "stderr is not sent through parent"

# pass env
[ -e _testoutput ] && rm _testoutput
FOO="howdydo" "$SCRIPT" --stdout=_testoutput ./samplechild.sh 0 0
grep "samplechild FOO=howdydo" _testoutput || addFailure "should have passed FOO environment variable"

# passes flags to subcommand
[ -e _testoutput ] && rm _testoutput
"$SCRIPT" --stdout=_testoutput -- ./samplechild.sh 0 0 --extra --args
grep "samplechild args 0 0 --extra --args" _testoutput || addFailure "should pass along args"

# pass SIGINT
"$SCRIPT" --stdout=_testoutput ./samplechild.sh 3 &
P=$!
sleep 1
kill -INT "$P"
wait "$P"
grep "samplechild got SIGINT" _testoutput || addFailure "should have passed SIGINT to child"

# pass SIGTERM
"$SCRIPT" --stdout=_testoutput ./samplechild.sh 3 &
P=$!
sleep 1
kill -TERM "$P"
wait "$P"
grep "samplechild got SIGTERM" _testoutput || addFailure "should have passed SIGTERM to child"

# should fail if only --start-time or --stop-time are given
"$SCRIPT" --start-time=05:00 ./samplechild.sh 0 0
RC=$?
[ ! "$RC" -eq "0" ] || addFailure "should fail to run if only --start-time is given"
"$SCRIPT" --stop-time=05:00 ./samplechild.sh 0 0
RC=$?
[ ! "$RC" -eq "0" ] || addFailure "should fail to run if only --stop-time is given"

# should start and stop
getStartStop() {
    cat <<EOF | python
import sys
from datetime import datetime, timedelta
now = datetime.utcnow()
next_minute = now + timedelta(minutes=1)
two_minutes = now + timedelta(minutes=2)
sys.stdout.write('{0} {1}\n'.format(
    next_minute.strftime('%H:%M'),
    two_minutes.strftime('%H:%M'),
))
EOF
}

TIMES=$(getStartStop)
STOP=$(echo $TIMES | cut -d' ' -f1)
START=$(echo $TIMES | cut -d' ' -f2)
echo "running long test..."
[ -e _testoutput ] && rm _testoutput
"$SCRIPT" --start-time="$START" --stop-time="$STOP" --stdout=_testoutput ./samplechild.sh 70 0
cat _testoutput
TOOK=$(grep "samplechild took" _testoutput | cut -d" " -f3)
echo "took: $TOOK"
[ "$TOOK" -gt "129" ] || addFailure "should have taken 70s plus the 60s stop time"


if [ -e _testfailures ]; then
    echo "FAILURES:"
    cat _testfailures
    exit 1
else
    echo "ok: all tests passed"
fi
