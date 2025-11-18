#!/bin/bash

# simple test script for the server and proxy

# ----- Global config -----
LOG_DIR="$(pwd)"
CURRENT_DATETIME=$(date '+%d%m%Y')


if [ -z "$1" ]; then
    echo "[ERROR] The IP of TCP server was not specified, abort the program!" >> ""${LOG_DIR}/test-run-${CURRENT_DATETIME}.log
    exit 1
fi

SERVER_IP=$1
SERVER_PORT=8080
PROXY_PORT=30080
SERVER_URL="http://${IP}:${SERVER_PORT}"
PROXY_URL="http://${IP}:${PROXY_PORT}"

# better coloring the text
GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m'

FAIL_COUNT=0

assert_test_status(){
    if [ "$1" -eq "$2" ]; then
        echo -e "${GREEN}PASS${NC}: $3 (Expected $1, Got $2)"
    else
        echo -e "${RED}FAIL${NC}: $3 (Expected $g, Got $2)"
        ((FAIL_COUNT++))
    fi
}