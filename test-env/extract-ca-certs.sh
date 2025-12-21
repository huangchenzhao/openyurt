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
CERT_DIR="./test-env/certs"

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
    
    # Check if kubectl is installed
    if ! command -v kubectl &> /dev/null; then
        error "kubectl 未安装。请先安装 kubectl"
    fi
    
    # Check if cluster exists
    if ! kubectl config get-contexts | grep -q "kind-$CLUSTER_NAME"; then
        error "集群 kind-$CLUSTER_NAME 不存在。请先运行 setup-kind-cluster.sh"
    fi
    
    # Create cert directory if it doesn't exist
    mkdir -p "$CERT_DIR"
    
    success "所有先决条件检查通过"
}

# Extract CA certificates from kind cluster
extract_ca_certs() {
    info "从 kind 集群提取 CA 证书..."
    
    # Get the control-plane container name
    CONTAINER_NAME=$(docker ps --filter "name=${CLUSTER_NAME}-control-plane" --format "{{.Names}}")
    
    if [ -z "$CONTAINER_NAME" ]; then
        error "找不到 kind 集群控制平面容器"
    fi
    
    info "找到控制平面容器: $CONTAINER_NAME"
    
    # Copy CA certificate from container
    docker cp "$CONTAINER_NAME:/etc/kubernetes/pki/ca.crt" "$CERT_DIR/ca.crt"
    
    # Copy CA key from container
    docker cp "$CONTAINER_NAME:/etc/kubernetes/pki/ca.key" "$CERT_DIR/ca.key"
    
    # Verify the certificates were extracted
    if [ ! -f "$CERT_DIR/ca.crt" ] || [ ! -f "$CERT_DIR/ca.key" ]; then
        error "提取 CA 证书失败"
    fi
    
    success "CA 证书提取成功"
}

# Verify certificates
verify_certs() {
    info "验证提取的证书..."
    
    # Check if certificate is valid
    if openssl x509 -in "$CERT_DIR/ca.crt" -text -noout > /dev/null 2>&1; then
        success "CA 证书验证成功"
    else
        error "CA 证书无效"
    fi
    
    # Show certificate details
    info "CA 证书详情:"
    openssl x509 -in "$CERT_DIR/ca.crt" -text -noout | grep -E "(Subject:|Issuer:|Not Before:|Not After:)"
}

# Main function
main() {
    info "开始提取主机集群 CA 证书..."
    
    check_prerequisites
    extract_ca_certs
    verify_certs
    
    success "CA 证书提取完成！"
    info "证书保存在: $CERT_DIR"
    info "  - ca.crt: CA 证书"
    info "  - ca.key: CA 私钥"
}

# Run main function
main "$@"