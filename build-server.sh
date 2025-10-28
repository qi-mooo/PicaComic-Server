#!/bin/bash

# PicaComic Server 构建脚本
# 只构建 macOS 和 Linux x86_64

set -e

echo "=================================="
echo "PicaComic Server 构建"
echo "=================================="
echo ""

# 检查 Go 是否安装
if ! command -v go &> /dev/null; then
    echo "❌ 错误: 未找到 Go，请先安装 Go"
    exit 1
fi

echo "📋 Go 版本:"
go version
echo ""

# 创建输出目录
OUTPUT_DIR="./bin"
mkdir -p $OUTPUT_DIR

# 获取版本信息
VERSION=$(date +%Y%m%d_%H%M%S)
echo "📌 版本: $VERSION"
echo ""

# 下载依赖
echo "📦 下载依赖..."
go mod download
echo "✅ 依赖下载完成"
echo ""

# 构建 macOS 版本
echo "🍎 构建 macOS (darwin/amd64)..."
GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o $OUTPUT_DIR/pica-server-darwin-amd64 main.go
echo "✅ macOS (amd64) 构建完成"
echo ""

echo "🍎 构建 macOS (darwin/arm64)..."
GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o $OUTPUT_DIR/pica-server-darwin-arm64 main.go
echo "✅ macOS (arm64) 构建完成"
echo ""

# 构建 Linux 版本
echo "🐧 构建 Linux (linux/amd64)..."
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o $OUTPUT_DIR/pica-server-linux-amd64 main.go
echo "✅ Linux (amd64) 构建完成"
echo ""

echo "📦 压缩文件..."
cd $OUTPUT_DIR

# 压缩 macOS 版本
echo "压缩 macOS (amd64)..."
tar -czf pica-server-darwin-amd64.tar.gz pica-server-darwin-amd64
echo "压缩 macOS (arm64)..."
tar -czf pica-server-darwin-arm64.tar.gz pica-server-darwin-arm64

# 压缩 Linux 版本
echo "压缩 Linux (amd64)..."
tar -czf pica-server-linux-amd64.tar.gz pica-server-linux-amd64

cd ..
echo ""

echo "✅ 构建完成!"
echo ""
echo "📊 构建产物:"
ls -lh $OUTPUT_DIR/
echo ""

echo "📦 可执行文件:"
echo "  - macOS (Intel):  $OUTPUT_DIR/pica-server-darwin-amd64"
echo "  - macOS (Apple):  $OUTPUT_DIR/pica-server-darwin-arm64"
echo "  - Linux (x86_64): $OUTPUT_DIR/pica-server-linux-amd64"
echo ""

echo "📦 压缩包:"
echo "  - macOS (Intel):  $OUTPUT_DIR/pica-server-darwin-amd64.tar.gz"
echo "  - macOS (Apple):  $OUTPUT_DIR/pica-server-darwin-arm64.tar.gz"
echo "  - Linux (x86_64): $OUTPUT_DIR/pica-server-linux-amd64.tar.gz"
echo ""

echo "🎉 全部完成！"

