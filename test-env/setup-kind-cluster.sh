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

# Configuration
CLUSTER_NAME="tenantcluster-test"
KIND_CONFIG="kind-config.yaml"
DATA_DIR="./test-env/data"

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

# Check prerequisites
check_prerequisites() {
    info "检查先决条件..."
    
    # Check if kind is installed
    if ! command -v kind &> /dev/null; then
        error "kind 未安装。请先安装 kind: https://kind.sigs.k8s.io/docs/user/quick-start/"
    fi
    
    # Check if kubectl is installed
    if ! command -v kubectl &> /dev/null; then
        error "kubectl 未安装。请先安装 kubectl"
    fi
    
    # Check if docker is running
    if ! docker info &> /dev/null; then
        error "Docker 未运行。请启动 Docker"
    fi
    
    # Create data directory if it doesn't exist
    mkdir -p "$DATA_DIR"
    
    success "所有先决条件检查通过"
}

# Create kind cluster
create_cluster() {
    info "创建 kind 集群: $CLUSTER_NAME"
    
    # Check if cluster already exists
    if kind get clusters | grep -q "$CLUSTER_NAME"; then
        warning "集群 $CLUSTER_NAME 已存在，删除旧集群..."
        kind delete cluster --name "$CLUSTER_NAME"
    fi
    
    # Create new cluster
    kind create cluster --config "$KIND_CONFIG" --name "$CLUSTER_NAME" --wait 300s
    
    success "kind 集群 $CLUSTER_NAME 创建成功"
}

# Configure kubectl
configure_kubectl() {
    info "配置 kubectl 上下文..."
    
    # Set kubectl context to the new cluster
    kubectl cluster-info --context kind-"$CLUSTER_NAME"
    
    success "kubectl 配置完成"
}

# Verify cluster
verify_cluster() {
    info "验证集群状态..."
    
    # Check nodes
    kubectl get nodes -o wide
    
    # Check cluster info
    kubectl cluster-info
    
    # Check all system pods are running
    kubectl get pods -n kube-system
    
    success "集群验证完成"
}

# Main function
main() {
    info "开始设置 TenantCluster 测试环境..."
    
    check_prerequisites
    create_cluster
    configure_kubectl
    verify_cluster
    
    success "kind 集群设置完成！"
    info "可以使用以下命令连接到集群:"
    info "  kubectl config use-context kind-$CLUSTER_NAME"
}

# Run main function
main "$@"