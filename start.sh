#!/bin/bash
# Radio.cn CLI Player - Start Script

echo "📻 Starting Radio.cn CLI Player..."

# Check if binary exists
if [ ! -f "radio-cmd" ]; then
    echo "Building radio-cmd..."
    go build -o radio-cmd ./cmd/radio-cmd
fi

# Run the application
./radio-cmd

# Keep the terminal open after exit (for debugging)
echo "\nPress Enter to exit..."
read
