#!/bin/bash

set -e 

if [ "$EUID" -ne 0 ]; then 
    echo -e "Внимание: Скрипт должен быть запущен от root.$"
    exit 1
fi

set -o allexport
ENV_FILE=".env"
source "$ENV_FILE"
set +o allexport

./main
