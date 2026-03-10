#!/bin/bash
# run_prod.sh - Production run using podman-compose

if [ ! -f ".env" ]; then
    if [ -f ".env.example" ]; then
        cp .env.example .env
        echo "Created .env from .env.example. Please configure it."
    else
        echo "Error: .env file not found."
        exit 1
    fi
fi

if [ ! -d "models" ] && [ -f "models.tar.gz.part_aa" ]; then
    echo "Extracting model parts..."
    cat models.tar.gz.part_* > models.tar.gz
    tar -xvzf models.tar.gz
    rm models.tar.gz
fi

podman-compose up -d --build
echo "Services started in production mode."
