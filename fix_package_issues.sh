#!/bin/bash

echo "===== 修复Go包结构问题 ====="

# 1. 修复examples/workflows目录中的包声明问题
echo "1. 修复examples/workflows目录中的混合包问题"

echo "创建子目录区分不同包..."
mkdir -p examples/workflows/main_examples
mkdir -p examples/workflows/workflow_examples

# 移动不同包的文件到对应子目录
echo "移动main包文件到main_examples目录..."
mv examples/workflows/chain_of_thought_example.go examples/workflows/main_examples/
mv examples/workflows/advanced_workflow_example.go examples/workflows/main_examples/
mv examples/workflows/parallel_cot_example.go examples/workflows/main_examples/
mv examples/workflows/parallel_workflow_example.go examples/workflows/main_examples/

echo "移动workflows包文件到workflow_examples目录..."
mv examples/workflows/event_workflow_example.go examples/workflows/workflow_examples/

echo "移动parallel_workflow_example包文件到单独目录..."
mkdir -p examples/workflows/parallel_workflow_examples
mv examples/workflows/parallel_workflow_example_fixed.go examples/workflows/parallel_workflow_examples/

# 2. 修复examples/agent_network目录中的main函数重复声明问题
echo "2. 检查agent_network目录中的main函数重复声明问题"

echo "查找所有包含main函数的文件..."
MAIN_FILES=$(grep -l "func main()" examples/agent_network/*.go)

if [ ! -z "$MAIN_FILES" ]; then
  echo "在examples/agent_network目录中找到以下包含main函数的文件:"
  echo "$MAIN_FILES"
  
  # 创建子目录存放不同的main示例
  echo "创建子目录区分不同示例..."
  mkdir -p examples/agent_network/examples
  
  # 保留context_sharing_example.go，移动其他main文件
  for file in $MAIN_FILES; do
    if [[ "$file" != *"context_sharing_example.go"* ]]; then
      echo "移动 $file 到 examples/agent_network/examples/ 目录..."
      filename=$(basename "$file")
      mv "$file" "examples/agent_network/examples/$filename"
    fi
  done
fi

echo "3. 运行go mod tidy更新依赖"
go mod tidy -e

echo "完成! 请检查是否还有剩余的包问题。" 