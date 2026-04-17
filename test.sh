#!/bin/bash
# Basic functionality test for radio-cmd

echo "========== Radio.cn CLI Player - Test Script =========="
echo ""

# Check Go version
echo "1. Checking Go installation..."
go version
GO_VERSION=$?
if [ $GO_VERSION -eq 0 ]; then
    echo "   ✓ Go is installed"
else
    echo "   ✗ Go is not installed or not in PATH"
    exit 1
fi
echo ""

# Check build
echo "2. Building application..."
go build -o radio-cmd ./cmd/radio-cmd
BUILD_STATUS=$?
if [ $BUILD_STATUS -eq 0 ]; then
    echo "   ✓ Build successful"
else
    echo "   ✗ Build failed"
    exit 1
fi
echo ""

# Check binary exists
echo "3. Checking binary..."
if [ -f "radio-cmd" ]; then
    echo "   ✓ radio-cmd binary created"
    ls -lh radio-cmd
else
    echo "   ✗ radio-cmd binary not found"
    exit 1
fi
echo ""

# Check for audio players
echo "4. Checking available audio players..."
if command -v mpv &> /dev/null; then
    echo "   ✓ mpv is installed"
else
    echo "   ⚠ mpv not found (required for playback)"
fi

if command -v ffplay &> /dev/null; then
    echo "   ✓ ffplay is installed"
else
    echo "   ⚠ ffplay not found"
fi

if command -v vlc &> /dev/null; then
    echo "   ✓ vlc is installed"
else
    echo "   ⚠ vlc not found"
fi
echo ""

# Check project structure
echo "5. Checking project structure..."
if [ -d "pkg/radio" ] && [ -d "pkg/tui" ] && [ -d "cmd/radio-cmd" ]; then
    echo "   ✓ Project structure is correct"
else
    echo "   ✗ Project structure is incomplete"
    exit 1
fi
echo ""

echo "========== Test Summary =========="
echo "Build: SUCCESS"
echo "Binary: radio-cmd (9-10 MB)"
echo "Ready to use: Run ./radio-cmd (requires mpv/ffplay/vlc for playback)"
echo ""
echo "To run the application:"
echo "  ./radio-cmd"
echo ""
echo "To see logs:"
echo "  tail -f ~/.radio-cmd/player.log"
