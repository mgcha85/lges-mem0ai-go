#!/bin/bash
# run_dev.sh - Development run (local build and run)

if [ ! -d "models" ] && [ -f "models.tar.gz.part_aa" ]; then
    echo "Extracting model parts..."
    cat models.tar.gz.part_* > models.tar.gz
    tar -xvzf models.tar.gz
    rm models.tar.gz
fi

export LD_LIBRARY_PATH=$LD_LIBRARY_PATH:$(pwd)/lib
go run cmd/server/main.go
