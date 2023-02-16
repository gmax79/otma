#!/bin/bash
psql postgresql://test:password@db:5432 -f /init/initdb.sql
