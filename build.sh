#!/bin/bash

# MySQL Backup System Build Script

APP_NAME="mysql-backup"
VERSION="1.0.0"
BUILD_DIR="dist"

echo "Building MySQL Backup System v${VERSION}"

# Clean previous builds
rm -rf ${BUILD_DIR}
mkdir -p ${BUILD_DIR}

# Build for different platforms
echo "Building for Linux (amd64)..."
GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -o ${BUILD_DIR}/${APP_NAME}-linux-amd64 main.go

echo "Building for Linux (arm64)..."
GOOS=linux GOARCH=arm64 go build -ldflags "-s -w" -o ${BUILD_DIR}/${APP_NAME}-linux-arm64 main.go

echo "Building for Windows (amd64)..."
GOOS=windows GOARCH=amd64 go build -ldflags "-s -w" -o ${BUILD_DIR}/${APP_NAME}-windows-amd64.exe main.go

echo "Building for macOS (amd64)..."
GOOS=darwin GOARCH=amd64 go build -ldflags "-s -w" -o ${BUILD_DIR}/${APP_NAME}-darwin-amd64 main.go

echo "Building for macOS (arm64)..."
GOOS=darwin GOARCH=arm64 go build -ldflags "-s -w" -o ${BUILD_DIR}/${APP_NAME}-darwin-arm64 main.go

echo "Build completed! Binaries are in the ${BUILD_DIR} directory."
echo ""
echo "Usage:"
echo "  Linux:   ./${BUILD_DIR}/${APP_NAME}-linux-amd64"
echo "  Windows: ./${BUILD_DIR}/${APP_NAME}-windows-amd64.exe"
echo "  macOS:   ./${BUILD_DIR}/${APP_NAME}-darwin-amd64"
