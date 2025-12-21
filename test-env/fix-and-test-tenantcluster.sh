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
set -o pipefail

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

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

# 默认配置
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
OPENYURT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
KIND_CLUSTER_NAME="${KIND_CLUSTER_NAME:-tenantcluster-test}"
IMAGE_REPO="${IMAGE_REPO:-openyurt}"
IMAGE_TAG="${IMAGE_TAG:-latest}"
NAMESPACE="${NAMESPACE:-openyurt-system}"
TEST_NAME="${TEST_NAME:-tc-test-$(date +%s)}"
CLEANUP="${CLEANUP:-true}"
SKIP_BUILD="${SKIP_BUILD:-false}"
SKIP_BASIC="${SKIP_BASIC:-false}"
SKIP_ADVANCED="${SKIP_ADVANCED:-false}"
REPORT_DIR="${REPORT_DIR:-$(pwd)/test-reports}"

# 显示帮助信息
show_help() {
    cat << EOF
用法: $0 [选项]

修复 TenantCluster operator 配置并测试功能

选项:
    -h, --help                          显示此帮助信息
    -c, --cleanup                       测试完成后清理资源
    --no-cleanup                        测试完成后不清理资源
    -b, --skip-build                    跳过镜像构建
    -n, --name <name>                   测试名称前缀 (默认: tc-test-<timestamp>)
    --skip-basic                        跳过基本功能测试
    --skip-advanced                     跳过高级功能测试
    --report-dir <dir>                  测试报告目录 (默认: ./test-reports)
    --cluster-name <name>               Kind 集群名称 (默认: tenantcluster-test)
    --repo <repo>                       镜像仓库 (默认: openyurt)
    --tag <tag>                         镜像标签 (默认: latest)

示例:
    $0                                  # 运行完整修复和测试流程
    $0 --skip-build                     # 跳过镜像构建
    $0 --no-cleanup                     # 测试后保留资源
    $0 --skip-advanced                  # 只运行基本测试
EOF
}

# 解析命令行参数
parse_args() {
    CLEANUP="true"  # 默认清理
    
    while [[ $# -gt 0 ]]; do
        case $1 in
            -h|--help)
                show_help
                exit 0
                ;;
            -c|--cleanup)
                CLEANUP="true"
                shift
                ;;
            --no-cleanup)
                CLEANUP="false"
                shift
                ;;
            -b|--skip-build)
                SKIP_BUILD="true"
                shift
                ;;
            -n|--name)
                TEST_NAME="$2"
                shift 2
                ;;
            --skip-basic)
                SKIP_BASIC="true"
                shift
                ;;
            --skip-advanced)
                SKIP_ADVANCED="true"
                shift
                ;;
            --report-dir)
                REPORT_DIR="$2"
                shift 2
                ;;
            --cluster-name)
                KIND_CLUSTER_NAME="$2"
                shift 2
                ;;
            --repo)
                IMAGE_REPO="$2"
                shift 2
                ;;
            --tag)
                IMAGE_TAG="$2"
                shift 2
                ;;
            *)
                log_error "未知参数: $1"
                show_help
                exit 1
                ;;
        esac
    done
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

# 检查 Kind 集群
check_kind_cluster() {
    log_info "检查 Kind 集群: ${KIND_CLUSTER_NAME}"
    
    if ! kind get clusters | grep -q "^${KIND_CLUSTER_NAME}$"; then
        log_error "Kind 集群 '${KIND_CLUSTER_NAME}' 不存在"
        log_info "请先运行 'test-env/setup-kind-cluster.sh' 创建集群"
        exit 1
    fi
    
    # 设置 kubectl 上下文
    kubectl config use-context "kind-${KIND_CLUSTER_NAME}"
    
    log_success "Kind 集群检查通过"
}

# 检查 CRD 是否存在
check_crds() {
    log_info "检查 CRD..."
    
    if ! kubectl get crd tenantclusters.apps.openyurt.io &>/dev/null; then
        log_warning "TenantCluster CRD 不存在，将部署 CRD"
        kubectl apply -f "${OPENYURT_ROOT}/charts/yurt-manager/crds/apiextensions.k8s.io_v1_customresourcedefinition_tenantclusters.apps.openyurt.io.yaml"
        
        # 等待 CRD 就绪
        local max_wait=30
        local wait_time=0
        while ! kubectl get crd tenantclusters.apps.openyurt.io &>/dev/null; do
            if [ $wait_time -ge $max_wait ]; then
                log_error "CRD 就绪等待超时"
                exit 1
            fi
            sleep 1
            ((wait_time++))
        done
    fi
    
    log_success "CRD 检查通过"
}

# 修复 operator 配置
fix_operator() {
    log_info "修复 TenantCluster operator 配置..."
    
    # 设置环境变量
    export KIND_CLUSTER_NAME="${KIND_CLUSTER_NAME}"
    export IMAGE_REPO="${IMAGE_REPO}"
    export IMAGE_TAG="${IMAGE_TAG}"
    export NAMESPACE="${NAMESPACE}"
    export SKIP_BUILD="${SKIP_BUILD}"
    
    # 执行修复脚本
    if ! bash "${SCRIPT_DIR}/fix-operator-config.sh" --cleanup --cluster-name "${KIND_CLUSTER_NAME}" --repo "${IMAGE_REPO}" --tag "${IMAGE_TAG}" --namespace "${NAMESPACE}" ${SKIP_BUILD:+--skip-build}; then
        log_error "Operator 配置修复失败"
        exit 1
    fi
    
    log_success "Operator 配置修复完成"
}

# 测试 TenantCluster 功能
test_tenantcluster() {
    log_info "测试 TenantCluster 功能..."
    
    # 设置环境变量
    export KIND_CLUSTER_NAME="${KIND_CLUSTER_NAME}"
    export NAMESPACE="${NAMESPACE}"
    export TEST_NAME="${TEST_NAME}"
    export CLEANUP="${CLEANUP}"
    export SKIP_BASIC="${SKIP_BASIC}"
    export SKIP_ADVANCED="${SKIP_ADVANCED}"
    export REPORT_DIR="${REPORT_DIR}"
    
    # 构建测试命令参数
    local test_args=""
    if [ "$CLEANUP" = "false" ]; then
        test_args="$test_args --no-cleanup"
    fi
    if [ "$SKIP_BASIC" = "true" ]; then
        test_args="$test_args --skip-basic"
    fi
    if [ "$SKIP_ADVANCED" = "true" ]; then
        test_args="$test_args --skip-advanced"
    fi
    test_args="$test_args --name ${TEST_NAME}"
    test_args="$test_args --report-dir ${REPORT_DIR}"
    test_args="$test_args --cluster-name ${KIND_CLUSTER_NAME}"
    
    # 执行测试脚本
    if ! bash "${SCRIPT_DIR}/test-tenantcluster-fixed.sh" $test_args; then
        log_error "TenantCluster 功能测试失败"
        exit 1
    fi
    
    log_success "TenantCluster 功能测试完成"
}

# 显示结果摘要
show_summary() {
    echo ""
    echo "======================================"
    echo "修复和测试完成"
    echo "======================================"
    echo "集群名称: ${KIND_CLUSTER_NAME}"
    echo "测试名称: ${TEST_NAME}"
    echo "命名空间: ${NAMESPACE}"
    echo "镜像: ${IMAGE_REPO}/yurt-manager:${IMAGE_TAG}"
    echo ""
    
    if [ "$CLEANUP" = "false" ]; then
        log_warning "资源保留模式：测试资源未清理"
        log_info "查看测试资源："
        echo "  kubectl get tenantcluster,all --all-namespaces | grep ${TEST_NAME}"
        log_info "清理资源："
        echo "  kubectl delete tenantcluster ${TEST_NAME} -n ${TEST_NAME}"
        echo "  kubectl delete namespace ${TEST_NAME}"
    fi
    
    log_info "查看测试报告："
    echo "  ls -la ${REPORT_DIR}/"
    echo "  cat \$(ls -t ${REPORT_DIR}/tenantcluster-test-*.md | head -1)"
    
    echo ""
    log_info "查看 operator 日志："
    echo "  kubectl logs -n ${NAMESPACE} -l app=yurt-manager -f"
}

# 主函数
main() {
    log_info "开始修复和测试 TenantCluster..."
    
    # 解析参数
    parse_args "$@"
    
    # 检查依赖
    check_dependencies
    
    # 检查 Kind 集群
    check_kind_cluster
    
    # 检查 CRD
    check_crds
    
    # 修复 operator 配置
    fix_operator
    
    # 测试 TenantCluster 功能
    test_tenantcluster
    
    # 显示结果摘要
    show_summary
    
    log_success "TenantCluster 修复和测试流程完成!"
}

# 错误处理
trap 'log_error "脚本执行失败，退出码: $?"' ERR

# 执行主函数
main "$@"