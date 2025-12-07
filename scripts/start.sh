#!/bin/bash

# MCP LocalBridge Startup Script

set -e

# Default configuration path
CONFIG_PATH="${CONFIG_PATH:-config/config.yaml}"

# Check if configuration file exists
if [ ! -f "$CONFIG_PATH" ]; then
    echo "Error: Configuration file not found: $CONFIG_PATH"
    exit 1
fi

echo "Starting MCP LocalBridge..."
echo "Configuration: $CONFIG_PATH"

# Check if running in Docker
if [ -f "/.dockerenv" ]; then
    echo "Running in Docker container"
    exec /app/mcp-server -config "$CONFIG_PATH"
else
    echo "Running locally"

    # Check if binary exists
    if [ -f "./bin/mcp-server" ]; then
        exec ./bin/mcp-server -config "$CONFIG_PATH"
    elif [ -f "./mcp-server" ]; then
        exec ./mcp-server -config "$CONFIG_PATH"
    else
        echo "Binary not found. Building..."
        make build-server
        exec ./bin/mcp-server -config "$CONFIG_PATH"
    fi
fi
