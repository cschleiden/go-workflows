#!/bin/bash

# Apply migrations
# If you get an error
migrate -database sqlite3://db.sqlite -path ./migrations up

# Dump schema
sqlite3 db.sqlite .schema > schema.sql

# Remove database
rm db.sqlite