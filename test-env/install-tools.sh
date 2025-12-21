#!/bin/bash

# Copyright 2025 The OpenYurt Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Versions
KUBECTL_VERSION="v1.32.0"
KIND_VERSION="v0.24.0"

# Helper functions
info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

error() {
    echo -e "${RED}[ERROR]${NC} $1"
    exit 1
}

# Check if running as root for system-wide installation
check_root() {
    if [[ $EUID -eq 0 ]]; then
        INSTALL_DIR="/usr/local/bin"
    else
        INSTALL_DIR="$HOME/.local/bin"
        mkdir -p "$INSTALL_DIR"
        
        # Add to PATH if not already there
        if [[ ":$PATH:" != *":$INSTALL_DIR:"* ]]; then
            echo "export PATH=\"\$PATH:$INSTALL_DIR\"" >> "$HOME/.bashrc"
            export PATH="$PATH:$INSTALL_DIR"
            warning "已将 $INSTALL_DIR 添加到 PATH，请重新加载终端或运行: source ~/.bashrc"
        fi
    fi
}

# Install kubectl
install_kubectl() {
    info "安装 kubectl..."
    
    if command -v kubectl &> /dev/null; then
        local current_version=$(kubectl version --client --short 2>/dev/null | grep -o 'v[0-9]\+\.[0-9]\+\.[0-9]\+' || echo "")
        if [[ "$current_version" == "$KUBECTL_VERSION" ]]; then
            success "kubectl $KUBECTL_VERSION 已安装"
            return 0
        else
            warning "kubectl 已安装但版本不匹配 (当前: $current_version, 需要: $KUBECTL_VERSION)"
        fi
    fi
    
    local os=$(uname | tr '[:upper:]' '[:lower:]')
    local arch=$(uname -m)
    case $arch in
        x86_64)
            arch="amd64"
            ;;
        aarch64|arm64)
            arch="arm64"
            ;;
        *)
            error "不支持的架构: $arch"
            ;;
    esac
    
    local kubectl_url="https://dl.k8s.io/release/${KUBECTL_VERSION}/bin/${os}/${arch}/kubectl"
    
    info "从 $kubectl_url 下载 kubectl..."
    if command -v curl &> /dev/null; then
        curl -fsSL "$kubectl_url" -o "$INSTALL_DIR/kubectl"
    elif command -v wget &> /dev/null; then
        wget -O "$INSTALL_DIR/kubectl" "$kubectl_url"
    else
        error "需要 curl 或 wget 来下载 kubectl"
    fi
    
    chmod +x "$INSTALL_DIR/kubectl"
    success "kubectl 安装完成"
}

# Install kind
install_kind() {
    info "安装 kind..."
    
    if command -v kind &> /dev/null; then
        local current_version=$(kind version 2>/dev/null | grep -o 'v[0-9]\+\.[0-9]\+\.[0-9]\+' || echo "")
        if [[ "$current_version" == "$KIND_VERSION" ]]; then
            success "kind $KIND_VERSION 已安装"
            return 0
        else
            warning "kind 已安装但版本不匹配 (当前: $current_version, 需要: $KIND_VERSION)"
        fi
    fi
    
    local os=$(uname | tr '[:upper:]' '[:lower:]')
    local arch=$(uname -m)
    case $arch in
        x86_64)
            arch="amd64"
            ;;
        aarch64|arm64)
            arch="arm64"
            ;;
        *)
            error "不支持的架构: $arch"
            ;;
    esac
    
    local kind_url="https://kind.sigs.k8s.io/dl/${KIND_VERSION}/kind-${os}-${arch}"
    
    info "从 $kind_url 下载 kind..."
    if command -v curl &> /dev/null; then
        curl -fsSL "$kind_url" -o "$INSTALL_DIR/kind"
    elif command -v wget &> /dev/null; then
        wget -O "$INSTALL_DIR/kind" "$kind_url"
    else
        error "需要 curl 或 wget 来下载 kind"
    fi
    
    chmod +x "$INSTALL_DIR/kind"
    success "kind 安装完成"
}

# Check Docker
check_docker() {
    info "检查 Docker..."
    
    if command -v docker &> /dev/null; then
        if docker info &> /dev/null; then
            success "Docker 已安装并正在运行"
        else
            error "Docker 已安装但未运行。请启动 Docker 服务"
        fi
    else
        error "Docker 未安装。请先安装 Docker"
        info "Ubuntu/Debian: sudo apt-get update && sudo apt-get install docker.io"
        info "CentOS/RHEL: sudo yum install docker"
        info "macOS: 下载 Docker Desktop"
        info "Windows: 下载 Docker Desktop"
        exit 1
    fi
}

# Verify installations
verify_installations() {
    info "验证安装..."
    
    if command -v kubectl &> /dev/null; then
        local kubectl_version=$(kubectl version --client --short 2>/dev/null || echo "无法获取版本")
        success "kubectl: $kubectl_version"
    else
        error "kubectl 安装失败"
    fi
    
    if command -v kind &> /dev/null; then
        local kind_version=$(kind version 2>/dev/null | head -n1 || echo "无法获取版本")
        success "kind: $kind_version"
    else
        error "kind 安装失败"
    fi
}

# Show usage
show_usage() {
    cat <<EOF
用法: $0 [选项]

选项:
  -h, --help              显示此帮助信息
  -k, --kubectl-only      仅安装 kubectl
  -n, --kind-only         仅安装 kind
  -s, --skip-docker       跳过 Docker 检查
  -v, --verbose           详细输出

示例:
  $0                      # 安装所有工具
  $0 -k                   # 仅安装 kubectl
  $0 -n                   # 仅安装 kind
  $0 -s                   # 安装工具但跳过 Docker 检查
EOF
}

# Parse command line arguments
INSTALL_KUBECTL=true
INSTALL_KIND=true
CHECK_DOCKER=true
VERBOSE=false

while [[ $# -gt 0 ]]; do
    case $1 in
        -h|--help)
            show_usage
            exit 0
            ;;
        -k|--kubectl-only)
            INSTALL_KIND=false
            shift
            ;;
        -n|--kind-only)
            INSTALL_KUBECTL=false
            shift
            ;;
        -s|--skip-docker)
            CHECK_DOCKER=false
            shift
            ;;
        -v|--verbose)
            VERBOSE=true
            shift
            ;;
        *)
            error "未知选项: $1。使用 -h 查看帮助。"
            ;;
    esac
done

# Main function
main() {
    info "开始安装 TenantCluster 测试环境所需的工具..."
    
    check_root
    
    if [ "$CHECK_DOCKER" = true ]; then
        check_docker
    fi
    
    if [ "$INSTALL_KUBECTL" = true ]; then
        install_kubectl
    fi
    
    if [ "$INSTALL_KIND" = true ]; then
        install_kind
    fi
    
    verify_installations
    
    success "工具安装完成！"
    info "现在可以运行 ./setup-test-env.sh 来设置测试环境"
}

# Run main function
main "$@"