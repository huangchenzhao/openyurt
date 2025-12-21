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

# Script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

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

# Show usage
show_usage() {
    cat <<EOF
用法: $0 [选项]

选项:
  -h, --help              显示此帮助信息
  -c, --check-only        仅检查环境，不执行设置
  -s, --step <step>       执行特定步骤 (1-6)
  -v, --verbose           详细输出

步骤说明:
  1 - 创建 kind 集群
  2 - 提取 CA 证书
  3 - 创建 CA secret
  4 - 安装 local-path-provisioner
  5 - 检查环境
  6 - 生成报告

示例:
  $0                      # 执行所有步骤
  $0 -c                   # 仅检查环境
  $0 -s 1                 # 仅执行步骤 1
  $0 -s 1 -s 2 -s 3       # 执行步骤 1, 2, 3
EOF
}

# Parse command line arguments
CHECK_ONLY=false
VERBOSE=false
STEPS=()

while [[ $# -gt 0 ]]; do
    case $1 in
        -h|--help)
            show_usage
            exit 0
            ;;
        -c|--check-only)
            CHECK_ONLY=true
            shift
            ;;
        -s|--step)
            if [[ "$2" =~ ^[1-6]$ ]]; then
                STEPS+=("$2")
                shift 2
            else
                error "无效的步骤编号: $2。必须是 1-6 之间的数字。"
            fi
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

# Execute a script step
execute_step() {
    local step_num=$1
    local step_name=$2
    local script_name=$3
    
    info "执行步骤 $step_num: $step_name"
    
    if [ "$VERBOSE" = true ]; then
        bash "$SCRIPT_DIR/$script_name"
    else
        bash "$SCRIPT_DIR/$script_name" > /dev/null 2>&1
    fi
    
    success "步骤 $step_num 完成: $step_name"
}

# Main execution
main() {
    info "开始设置 TenantCluster 测试环境..."
    
    # Change to script directory
    cd "$SCRIPT_DIR"
    
    # Make all scripts executable
    chmod +x *.sh
    
    # If check-only mode, just run environment check
    if [ "$CHECK_ONLY" = true ]; then
        info "仅检查环境模式..."
        execute_step 5 "检查环境" "check-environment.sh"
        exit 0
    fi
    
    # If no specific steps specified, run all steps
    if [ ${#STEPS[@]} -eq 0 ]; then
        STEPS=(1 2 3 4 5 6)
    fi
    
    # Execute requested steps
    for step in "${STEPS[@]}"; do
        case $step in
            1)
                execute_step 1 "创建 kind 集群" "setup-kind-cluster.sh"
                ;;
            2)
                execute_step 2 "提取 CA 证书" "extract-ca-certs.sh"
                ;;
            3)
                execute_step 3 "创建 CA secret" "create-ca-secret.sh"
                ;;
            4)
                execute_step 4 "安装 local-path-provisioner" "install-local-path-provisioner.sh"
                ;;
            5)
                execute_step 5 "检查环境" "check-environment.sh"
                ;;
            6)
                execute_step 6 "生成报告" "check-environment.sh"
                ;;
            *)
                error "无效的步骤编号: $step"
                ;;
        esac
    done
    
    success "TenantCluster 测试环境设置完成！"
    info "详细报告已保存到: ./test-env/environment-report.txt"
    
    # Show next steps
    cat <<EOF

${GREEN}下一步操作:${NC}
1. 部署 TenantCluster CRD 和 operator
2. 创建测试 TenantCluster 资源
3. 验证 tenant 集群功能

${GREEN}示例 TenantCluster YAML:${NC}
apiVersion: apps.openyurt.io/v1alpha1
kind: TenantCluster
metadata:
  name: my-tenant
spec:
  kubernetesVersion: "v1.32.0"
  apiServerServiceIP: "10.96.100.1"
  apiServerNodePort: 31000
  hostControlPlaneAddr: "172.18.0.2:6443"
  hostCASecretRef:
    name: host-k8s-ca
    namespace: kube-system
  etcdStorageClass: "local-path"
  etcdStorageSize: "1Gi"
EOF
}

# Run main function
main "$@"