#!/bin/bash

cmd="${1}"
DIR="$(cd "$(dirname "$0")" && pwd)"
cd "${DIR}"
sf=$(basename $0)
servicename=getpics
pidfile="/var/run/${servicename}-run.pid"
pid=$(cat ${pidfile} 2>/dev/null)

usage() {
    echo "Usage: ${sf} [start|stop|status|restart]"
    exit 1
}

running() {
    [ -z "${pid}" ] && return 1
    $(kill -0 "${pid}" 2>/dev/null)
}

startup() {
    running
    if [ $? -eq 0 ]; then
        echo "already running: ${servicename}"
    else
        sh getpics.sh &
        ret=$?
        disown $! >/dev/null 2>&1
        echo $! >"${pidfile}"
        if [ ${ret} -ne 0 ]; then
            echo "start failed: ${servicename}"
        else
            echo "started: ${servicename}"
        fi
    fi
}

stopnow() {
    running
    if [ $? -eq 0 ]; then
        kill -9 ${pid}
        $(rm -f "${pidfile}" >/dev/null 2>&1)
        echo "stopped: ${servicename}"
    else
        echo "not running: ${servicename}"
    fi
}

case ${cmd} in
status)
    running
    if [ $? -eq 0 ]; then
        echo "running: ${servicename}"
    else
        echo "not running: ${servicename}"
        rm -f "${pidfile}" >/dev/null 2>&1
        exit 1
    fi
    ;;
start)
    startup
    ;;
stop)
    stopnow
    ;;
restart)
        sh ./"${sf}" stop
        sh ./"${sf}" start    
    ;;
*)
    usage
    ;;
esac

exit 0
