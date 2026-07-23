#!/bin/bash
set -e

# Radio.cn CLI Player - Start Script

echo "📻 Starting Radio.cn CLI Player..."

# Always build the current source so stale binaries cannot hide fixes
echo "Building radio-cmd..."
go build -o radio-cmd ./cmd/radio-cmd

# Run the application
./radio-cmd

# Keep the terminal open after exit (for debugging)
echo "\nPress Enter to exit..."
read
