#!/bin/bash
# stop.sh - Stop services

podman-compose down
echo "Services stopped."
