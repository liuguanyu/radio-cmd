#!/bin/bash
# 测试 subtitle 是否会实时更新

echo "测试 radio.cn API 的 subtitle 实时性"
echo "========================================"
echo ""

API_URL="https://ytmsout.radio.cn/web/appBroadcast/list?categoryId=0&provinceCode=0"

echo "第一次请求（当前时间：$(date '+%Y-%m-%d %H:%M:%S')）："
curl -s "$API_URL" | jq '.data[] | select(.contentId == "639") | {title, subtitle}'
echo ""
echo "等待10秒后再次请求..."
sleep 10

echo ""
echo "第二次请求（当前时间：$(date '+%Y-%m-%d %H:%M:%S')）："
curl -s "$API_URL" | jq '.data[] | select(.contentId == "639") | {title, subtitle}'
echo ""

echo "注意：如果两次请求的 subtitle 不同，说明是实时更新的"
