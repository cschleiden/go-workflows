#!/bin/bash

set -e
set +x

# Generate random DB name
DB_NAME=workflows_$(date +%s)

# Check if mysql container is running
if [ ! "$(docker ps -q -f name=go-workflows-db-1)" ]; then
    echo "MySQL container is not running"
    exit 1
fi

# Create database $DB_NAME
docker exec go-workflows-db-1 mysql -uroot -proot -e "CREATE DATABASE $DB_NAME"

# Run all migrations
migrate -database "mysql://root:root@tcp(127.0.0.1:3306)/$DB_NAME" -source file://./migrations up

# Dump schema
docker exec go-workflows-db-1 mysqldump -uroot -proot $DB_NAME | sed -e 's/^\/\*![0-9]\{5\}.*\/;$//g' > ./schema.sql

# Drop database $DB_NAME
docker exec go-workflows-db-1 mysql -uroot -proot -e "DROP DATABASE $DB_NAME"