#!/bin/sh
set -e

# Create staging and prod databases if they do not exist
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" <<-EOSQL
    CREATE DATABASE goapp_staging;
    CREATE DATABASE goapp_prod;
EOSQL
