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

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 脚本目录
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# 日志函数
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# 显示帮助信息
show_help() {
    cat << EOF
用法: $0 [选项]

TenantCluster 测试运行脚本 - 简化的测试执行流程

选项:
    -h, --help                          显示此帮助信息
    -q, --quick                         快速测试模式（仅基本功能）
    -f, --full                          完整测试模式（所有场景）
    -d, --debug                         调试模式（保留资源）
    -s, --setup                         自动设置测试环境
    -c, --cleanup                       清理测试环境

示例:
    $0                                  # 运行基本测试
    $0 -q                               # 快速测试模式
    $0 -f                               # 完整测试模式
    $0 -d                               # 调试模式
    $0 -s                               # 设置环境并测试
    $0 -c                               # 清理环境
EOF
}

# 解析命令行参数
parse_args() {
    QUICK_MODE=false
    FULL_MODE=false
    DEBUG_MODE=false
    SETUP_MODE=false
    CLEANUP_MODE=false
    
    while [[ $# -gt 0 ]]; do
        case $1 in
            -h|--help)
                show_help
                exit 0
                ;;
            -q|--quick)
                QUICK_MODE=true
                shift
                ;;
            -f|--full)
                FULL_MODE=true
                shift
                ;;
            -d|--debug)
                DEBUG_MODE=true
                shift
                ;;
            -s|--setup)
                SETUP_MODE=true
                shift
                ;;
            -c|--cleanup)
                CLEANUP_MODE=true
                shift
                ;;
            *)
                log_error "未知参数: $1"
                show_help
                exit 1
                ;;
        esac
    done
    
    # 设置默认模式
    if [ "$QUICK_MODE" = false ] && [ "$FULL_MODE" = false ] && [ "$SETUP_MODE" = false ] && [ "$CLEANUP_MODE" = false ]; then
        QUICK_MODE=true
    fi
}

# 检查依赖
check_dependencies() {
    log_info "检查依赖..."
    
    local missing_deps=()
    
    if ! command -v kubectl &> /dev/null; then
        missing_deps+=("kubectl")
    fi
    
    if ! command -v docker &> /dev/null; then
        missing_deps+=("docker")
    fi
    
    if ! command -v kind &> /dev/null; then
        missing_deps+=("kind")
    fi
    
    if [ ${#missing_deps[@]} -ne 0 ]; then
        log_error "缺少以下依赖: ${missing_deps[*]}"
        log_info "请运行 'test-env/install-tools.sh' 安装所需依赖"
        exit 1
    fi
    
    log_success "依赖检查通过"
}

# 设置测试环境
setup_test_environment() {
    log_info "设置测试环境..."
    
    # 检查并创建 Kind 集群
    if ! kind get clusters | grep -q "tenantcluster-test"; then
        log_info "创建 Kind 集群..."
        if ! bash "$SCRIPT_DIR/setup-kind-cluster.sh"; then
            log_error "Kind 集群创建失败"
            exit 1
        fi
    else
        log_info "Kind 集群已存在"
    fi
    
    # 提取 CA 证书
    log_info "提取 CA 证书..."
    if ! bash "$SCRIPT_DIR/extract-ca-certs.sh"; then
        log_error "CA 证书提取失败"
        exit 1
    fi
    
    # 创建 CA secret
    log_info "创建 CA secret..."
    if ! bash "$SCRIPT_DIR/create-ca-secret.sh"; then
        log_error "CA secret 创建失败"
        exit 1
    fi
    
    # 安装 local-path-provisioner
    log_info "安装 local-path-provisioner..."
    if ! bash "$SCRIPT_DIR/install-local-path-provisioner.sh"; then
        log_error "local-path-provisioner 安装失败"
        exit 1
    fi
    
    # 部署 operator
    log_info "部署 TenantCluster operator..."
    if ! bash "$SCRIPT_DIR/build-and-deploy-operator.sh" --skip-build; then
        log_error "Operator 部署失败"
        exit 1
    fi
    
    log_success "测试环境设置完成"
}

# 清理测试环境
cleanup_test_environment() {
    log_info "清理测试环境..."
    
    # 删除所有 TenantCluster
    log_info "删除 TenantCluster 资源..."
    kubectl get tenantcluster --no-headers 2>/dev/null | awk '{print $1}' | xargs -I {} kubectl delete tenantcluster {} --ignore-not-found=true || true
    
    # 删除测试命名空间
    log_info "删除测试命名空间..."
    kubectl get namespaces --no-headers 2>/dev/null | grep "^tc-test-" | awk '{print $1}' | xargs -I {} kubectl delete namespace {} --ignore-not-found=true || true
    
    # 删除 operator
    log_info "删除 operator..."
    bash "$SCRIPT_DIR/build-and-deploy-operator.sh" --cleanup --skip-build --skip-crd --skip-operator || true
    
    # 删除 Kind 集群
    log_info "删除 Kind 集群..."
    if kind get clusters | grep -q "tenantcluster-test"; then
        kind delete cluster --name tenantcluster-test || true
    fi
    
    log_success "测试环境清理完成"
}

# 运行快速测试
run_quick_test() {
    log_info "运行快速测试（基本功能）..."
    
    local test_args="--scenario basic"
    
    if [ "$DEBUG_MODE" = true ]; then
        test_args="$test_args --no-cleanup"
    fi
    
    if ! bash "$SCRIPT_DIR/test-tenantcluster.sh" $test_args; then
        log_error "快速测试失败"
        return 1
    fi
    
    log_success "快速测试通过"
}

# 运行完整测试
run_full_test() {
    log_info "运行完整测试（所有场景）..."
    
    local test_args=""
    
    if [ "$DEBUG_MODE" = true ]; then
        test_args="$test_args --no-cleanup"
    fi
    
    if ! bash "$SCRIPT_DIR/test-tenantcluster.sh" $test_args; then
        log_error "完整测试失败"
        return 1
    fi
    
    log_success "完整测试通过"
}

# 显示测试结果
show_results() {
    echo ""
    echo "======================================"
    echo "测试完成"
    echo "======================================"
    
    if [ "$DEBUG_MODE" = true ]; then
        log_warning "调试模式：测试资源已保留"
        log_info "查看测试资源："
        echo "  kubectl get tenantcluster,all --all-namespaces | grep tc-test"
        log_info "清理资源："
        echo "  $0 -c"
    fi
    
    log_info "查看测试报告："
    echo "  ls -la ./test-reports/"
}

# 主函数
main() {
    log_info "TenantCluster 测试运行器"
    
    # 解析参数
    parse_args "$@"
    
    # 检查依赖
    check_dependencies
    
    # 执行相应操作
    if [ "$CLEANUP_MODE" = true ]; then
        cleanup_test_environment
        exit 0
    fi
    
    if [ "$SETUP_MODE" = true ]; then
        setup_test_environment
    fi
    
    # 运行测试
    if [ "$QUICK_MODE" = true ]; then
        run_quick_test
    elif [ "$FULL_MODE" = true ]; then
        run_full_test
    fi
    
    # 显示结果
    show_results
}

# 执行主函数
main "$@"