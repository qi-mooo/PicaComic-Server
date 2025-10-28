#!/bin/bash

# 默认配置
PORT=8080
HOST="0.0.0.0"
DOWNLOAD_PATH=""

# 解析参数
while [[ $# -gt 0 ]]; do
    case $1 in
        -p|--port)
            PORT="$2"
            shift 2
            ;;
        -h|--host)
            HOST="$2"
            shift 2
            ;;
        -d|--download-path)
            DOWNLOAD_PATH="$2"
            shift 2
            ;;
        *)
            echo "未知参数: $1"
            echo "用法: $0 [-p|--port PORT] [-h|--host HOST] [-d|--download-path PATH]"
            exit 1
            ;;
    esac
done

echo "正在启动 PicaComic 服务器..."
echo "端口: $PORT"
echo "监听地址: $HOST"

# 构建命令
CMD="go run main.go -port $PORT -host $HOST"
if [ -n "$DOWNLOAD_PATH" ]; then
    CMD="$CMD -download-path $DOWNLOAD_PATH"
    echo "下载目录: $DOWNLOAD_PATH"
fi

echo ""
$CMD

