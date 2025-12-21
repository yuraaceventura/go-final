#!/bin/bash

set -e 

if [ "$EUID" -ne 0 ]; then 
    echo -e "Внимание: Скрипт должен быть запущен от root."
    exit 1
fi

/usr/local/go/bin/go mod download
/usr/local/go/bin/go mod tidy

ENV_FILE=".env"
source "$ENV_FILE"


if ! sudo -u postgres psql -tAc "SELECT 1 FROM pg_roles WHERE rolname='$POSTGRES_USER'" | grep -q 1; then
    sudo -u postgres psql -c "CREATE USER $POSTGRES_USER WITH PASSWORD '$POSTGRES_PASS';" 
else
    sudo -u postgres psql -c "ALTER USER $POSTGRES_USER WITH PASSWORD '$POSTGRES_PASS';" 
fi

if ! sudo -u postgres psql -lqt | cut -d \| -f 1 | grep -qw "$POSTGRES_DB"; then
    sudo -u postgres createdb "$POSTGRES_DB"
fi

sudo -u postgres psql -c "GRANT ALL PRIVILEGES ON DATABASE \"$POSTGRES_DB\" TO $POSTGRES_USER;"

sudo -u postgres psql -d "$POSTGRES_DB" -c "GRANT ALL ON SCHEMA public TO $POSTGRES_USER;"
sudo -u postgres psql -d "$POSTGRES_DB" -c "GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO $POSTGRES_USER;"
sudo -u postgres psql -d "$POSTGRES_DB" -c "GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO $POSTGRES_USER;"
sudo -u postgres psql -d "$POSTGRES_DB" -c "GRANT ALL PRIVILEGES ON ALL FUNCTIONS IN SCHEMA public TO $POSTGRES_USER;"

/usr/local/go/bin/go build -o main .