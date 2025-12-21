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
SECRET_NAME="host-k8s-ca"
SECRET_NAMESPACE="kube-system"

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
    
    # Check if certificate files exist
    if [ ! -f "$CERT_DIR/ca.crt" ] || [ ! -f "$CERT_DIR/ca.key" ]; then
        error "CA 证书文件不存在。请先运行 extract-ca-certs.sh"
    fi
    
    success "所有先决条件检查通过"
}

# Create or update the secret
create_ca_secret() {
    info "创建 host-k8s-ca secret..."
    
    # Set kubectl context to the kind cluster
    kubectl config use-context kind-"$CLUSTER_NAME"
    
    # Check if secret already exists
    if kubectl get secret "$SECRET_NAME" -n "$SECRET_NAMESPACE" &> /dev/null; then
        warning "Secret $SECRET_NAME 已存在于命名空间 $SECRET_NAMESPACE，将删除并重新创建..."
        kubectl delete secret "$SECRET_NAME" -n "$SECRET_NAMESPACE"
    fi
    
    # Create the secret
    kubectl create secret generic "$SECRET_NAME" \
        -n "$SECRET_NAMESPACE" \
        --from-file=ca.crt="$CERT_DIR/ca.crt" \
        --from-file=ca.key="$CERT_DIR/ca.key" \
        --dry-run=client -o yaml | kubectl apply -f -
    
    success "Secret $SECRET_NAME 创建成功"
}

# Verify the secret
verify_secret() {
    info "验证创建的 secret..."
    
    # Get secret details
    kubectl get secret "$SECRET_NAME" -n "$SECRET_NAMESPACE" -o yaml
    
    # Verify the secret contains the required keys
    CA_CRT=$(kubectl get secret "$SECRET_NAME" -n "$SECRET_NAMESPACE" -o jsonpath='{.data.ca\.crt}')
    CA_KEY=$(kubectl get secret "$SECRET_NAME" -n "$SECRET_NAMESPACE" -o jsonpath='{.data.ca\.key}')
    
    if [ -z "$CA_CRT" ] || [ -z "$CA_KEY" ]; then
        error "Secret 验证失败：缺少必要的证书文件"
    fi
    
    success "Secret 验证成功"
}

# Show secret information
show_secret_info() {
    info "Secret 信息:"
    info "  名称: $SECRET_NAME"
    info "  命名空间: $SECRET_NAMESPACE"
    info "  包含文件: ca.crt, ca.key"
    
    info "在 TenantCluster CR 中引用此 secret 的示例:"
    cat <<EOF
apiVersion: apps.openyurt.io/v1alpha1
kind: TenantCluster
metadata:
  name: my-tenant
spec:
  hostCASecretRef:
    name: $SECRET_NAME
    namespace: $SECRET_NAMESPACE
  # ... 其他配置
EOF
}

# Main function
main() {
    info "开始创建 host-k8s-ca secret..."
    
    check_prerequisites
    create_ca_secret
    verify_secret
    show_secret_info
    
    success "host-k8s-ca secret 创建完成！"
}

# Run main function
main "$@"