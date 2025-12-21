#!/bin/bash

# Copyright 2024 The OpenYurt Authors.
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
YELLOW='\033[0;33m'
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
KIND_CLUSTER_NAME="${KIND_CLUSTER_NAME:-openyurt}"
NAMESPACE="${NAMESPACE:-openyurt-system}"
IMAGE_REPO="${IMAGE_REPO:-openyurt}"
IMAGE_TAG="${IMAGE_TAG:-test}"
TEST_TIMEOUT="${TEST_TIMEOUT:-300}"  # 5 minutes

# 测试结果
TESTS_PASSED=0
TESTS_FAILED=0

# 显示帮助信息
show_help() {
    cat << EOF
用法: $0 [选项]

测试 TenantCluster operator 部署脚本的功能

选项:
    -h, --help                          显示此帮助信息
    -n, --namespace <namespace>         命名空间 (默认: openyurt-system)
    -r, --repo <repo>                   镜像仓库 (默认: openyurt)
    -t, --tag <tag>                     镜像标签 (默认: test)
    --cluster-name <name>               Kind 集群名称 (默认: openyurt)
    --timeout <seconds>                 测试超时时间 (默认: 300)

示例:
    $0                                  # 使用默认配置测试
    $0 -n my-namespace -t v1.0.0        # 指定命名空间和标签测试
EOF
}

# 解析命令行参数
parse_args() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            -h|--help)
                show_help
                exit 0
                ;;
            -n|--namespace)
                NAMESPACE="$2"
                shift 2
                ;;
            -r|--repo)
                IMAGE_REPO="$2"
                shift 2
                ;;
            -t|--tag)
                IMAGE_TAG="$2"
                shift 2
                ;;
            --cluster-name)
                KIND_CLUSTER_NAME="$2"
                shift 2
                ;;
            --timeout)
                TEST_TIMEOUT="$2"
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

# 测试函数
test_passed() {
    log_success "$1"
    ((TESTS_PASSED++))
}

test_failed() {
    log_error "$1"
    ((TESTS_FAILED++))
}

# 测试依赖
test_dependencies() {
    log_info "测试依赖..."
    
    if ! command -v kubectl &> /dev/null; then
        test_failed "kubectl 未安装"
        return 1
    fi
    
    if ! command -v docker &> /dev/null; then
        test_failed "docker 未安装"
        return 1
    fi
    
    if ! command -v kind &> /dev/null; then
        test_failed "kind 未安装"
        return 1
    fi
    
    test_passed "依赖检查通过"
}

# 测试 Kind 集群
test_kind_cluster() {
    log_info "测试 Kind 集群..."
    
    if ! kind get clusters | grep -q "^${KIND_CLUSTER_NAME}$"; then
        test_failed "Kind 集群 '${KIND_CLUSTER_NAME}' 不存在"
        return 1
    fi
    
    kubectl config use-context "kind-${KIND_CLUSTER_NAME}"
    
    test_passed "Kind 集群检查通过"
}

# 测试构建脚本
test_build_script() {
    log_info "测试构建脚本..."
    
    if [ ! -f "${SCRIPT_DIR}/build-and-deploy-operator.sh" ]; then
        test_failed "构建脚本不存在"
        return 1
    fi
    
    if [ ! -x "${SCRIPT_DIR}/build-and-deploy-operator.sh" ]; then
        chmod +x "${SCRIPT_DIR}/build-and-deploy-operator.sh"
    fi
    
    # 测试帮助信息
    if ! "${SCRIPT_DIR}/build-and-deploy-operator.sh" --help > /dev/null 2>&1; then
        test_failed "构建脚本帮助信息测试失败"
        return 1
    fi
    
    test_passed "构建脚本检查通过"
}

# 测试调试脚本
test_debug_script() {
    log_info "测试调试脚本..."
    
    if [ ! -f "${SCRIPT_DIR}/debug-operator.sh" ]; then
        test_failed "调试脚本不存在"
        return 1
    fi
    
    if [ ! -x "${SCRIPT_DIR}/debug-operator.sh" ]; then
        chmod +x "${SCRIPT_DIR}/debug-operator.sh"
    fi
    
    # 测试帮助信息
    if ! "${SCRIPT_DIR}/debug-operator.sh" --help > /dev/null 2>&1; then
        test_failed "调试脚本帮助信息测试失败"
        return 1
    fi
    
    test_passed "调试脚本检查通过"
}

# 测试部署
test_deployment() {
    log_info "测试部署..."
    
    # 清理现有资源
    log_info "清理现有资源..."
    "${SCRIPT_DIR}/build-and-deploy-operator.sh" --cleanup --skip-build --skip-crd --skip-operator -n "${NAMESPACE}" || true
    
    # 部署 CRD 和 operator (跳过构建以节省时间)
    log_info "部署 CRD 和 operator..."
    if ! "${SCRIPT_DIR}/build-and-deploy-operator.sh" --skip-build -n "${NAMESPACE}" -r "${IMAGE_REPO}" -t "${IMAGE_TAG}"; then
        test_failed "部署失败"
        return 1
    fi
    
    test_passed "部署检查通过"
}

# 测试 CRD
test_crd() {
    log_info "测试 CRD..."
    
    # 检查 CRD 是否存在
    local max_wait=30
    local wait_time=0
    while ! kubectl get crd tenantclusters.apps.openyurt.io &>/dev/null; do
        if [ $wait_time -ge $max_wait ]; then
            test_failed "CRD 注册超时"
            return 1
        fi
        sleep 1
        ((wait_time++))
    done
    
    # 检查 CRD 版本
    local crd_versions=$(kubectl get crd tenantclusters.apps.openyurt.io -o jsonpath='{.spec.versions[*].name}' 2>/dev/null || echo "")
    if [[ ! "${crd_versions}" =~ "v1alpha1" ]]; then
        test_failed "CRD 版本不正确: ${crd_versions}"
        return 1
    fi
    
    test_passed "CRD 检查通过"
}

# 测试 operator pod
test_operator_pod() {
    log_info "测试 operator pod..."
    
    # 等待 pod 启动
    local max_wait=120
    local wait_time=0
    while true; do
        local pod_status=$(kubectl get pods -n "${NAMESPACE}" -l app=yurt-manager -o jsonpath='{.items[0].status.phase}' 2>/dev/null || echo "")
        
        if [ "${pod_status}" = "Running" ]; then
            break
        elif [ "${pod_status}" = "Error" ] || [ "${pod_status}" = "CrashLoopBackOff" ]; then
            test_failed "Pod 处于错误状态: ${pod_status}"
            return 1
        fi
        
        if [ $wait_time -ge $max_wait ]; then
            test_failed "Pod 启动超时"
            return 1
        fi
        
        sleep 2
        ((wait_time++))
    done
    
    # 检查就绪状态
    local ready=$(kubectl get pods -n "${NAMESPACE}" -l app=yurt-manager -o jsonpath='{.items[0].status.conditions[?(@.type=="Ready")].status}' 2>/dev/null || echo "")
    if [ "${ready}" != "True" ]; then
        test_failed "Pod 未就绪"
        return 1
    fi
    
    test_passed "Operator pod 检查通过"
}

# 测试 TenantCluster controller
test_tenantcluster_controller() {
    log_info "测试 TenantCluster controller..."
    
    # 检查日志中是否有 controller 启动信息
    local max_wait=60
    local wait_time=0
    while true; do
        local controller_logs=$(kubectl logs -n "${NAMESPACE}" -l app=yurt-manager --tail=20 2>/dev/null || echo "")
        if echo "${controller_logs}" | grep -q "tenantcluster-controller"; then
            break
        fi
        
        if [ $wait_time -ge $max_wait ]; then
            test_failed "TenantCluster controller 启动超时"
            return 1
        fi
        
        sleep 2
        ((wait_time++))
    done
    
    test_passed "TenantCluster controller 检查通过"
}

# 测试 TenantCluster 资源创建
test_tenantcluster_creation() {
    log_info "测试 TenantCluster 资源创建..."
    
    # 检查示例文件是否存在
    if [ ! -f "${SCRIPT_DIR}/tenantcluster-sample.yaml" ]; then
        test_failed "TenantCluster 示例文件不存在"
        return 1
    fi
    
    # 创建测试 TenantCluster
    local test_tc_name="test-$(date +%s)"
    local temp_tc_file="/tmp/test-tenantcluster.yaml"
    
    # 复制并修改示例文件
    sed "s/name: tenantcluster-sample/name: ${test_tc_name}/g" "${SCRIPT_DIR}/tenantcluster-sample.yaml" > "${temp_tc_file}"
    
    # 应用 TenantCluster
    if ! kubectl apply -f "${temp_tc_file}"; then
        test_failed "TenantCluster 创建失败"
        rm -f "${temp_tc_file}"
        return 1
    fi
    
    # 等待 TenantCluster 处理
    sleep 10
    
    # 检查 TenantCluster 是否存在
    if ! kubectl get tenantcluster "${test_tc_name}"; then
        test_failed "TenantCluster 不存在"
        kubectl delete tenantcluster "${test_tc_name}" --ignore-not-found=true
        rm -f "${temp_tc_file}"
        return 1
    fi
    
    # 清理
    kubectl delete tenantcluster "${test_tc_name}" --ignore-not-found=true
    rm -f "${temp_tc_file}"
    
    test_passed "TenantCluster 资源创建检查通过"
}

# 测试调试功能
test_debug_functionality() {
    log_info "测试调试功能..."
    
    # 测试状态检查
    if ! "${SCRIPT_DIR}/debug-operator.sh" status -n "${NAMESPACE}" > /dev/null 2>&1; then
        test_failed "调试状态检查失败"
        return 1
    fi
    
    # 测试事件收集
    if ! "${SCRIPT_DIR}/debug-operator.sh" events -n "${NAMESPACE}" > /dev/null 2>&1; then
        test_failed "调试事件收集失败"
        return 1
    fi
    
    test_passed "调试功能检查通过"
}

# 清理测试资源
cleanup_test_resources() {
    log_info "清理测试资源..."
    
    # 删除测试 TenantCluster
    kubectl get tenantcluster --no-headers | awk '{print $1}' | xargs -I {} kubectl delete tenantcluster {} --ignore-not-found=true || true
    
    # 删除部署
    "${SCRIPT_DIR}/build-and-deploy-operator.sh" --cleanup --skip-build --skip-crd --skip-operator -n "${NAMESPACE}" || true
    
    log_info "测试资源清理完成"
}

# 显示测试结果
show_test_results() {
    echo ""
    echo "======================================"
    echo "测试结果汇总"
    echo "======================================"
    echo "通过: ${TESTS_PASSED}"
    echo "失败: ${TESTS_FAILED}"
    echo "总计: $((TESTS_PASSED + TESTS_FAILED))"
    
    if [ ${TESTS_FAILED} -eq 0 ]; then
        log_success "所有测试通过!"
        return 0
    else
        log_error "有 ${TESTS_FAILED} 个测试失败"
        return 1
    fi
}

# 主函数
main() {
    log_info "开始测试 TenantCluster operator 部署..."
    
    # 解析参数
    parse_args "$@"
    
    # 设置陷阱以清理资源
    trap cleanup_test_resources EXIT
    
    # 运行测试
    test_dependencies
    test_kind_cluster
    test_build_script
    test_debug_script
    test_deployment
    test_crd
    test_operator_pod
    test_tenantcluster_controller
    test_tenantcluster_creation
    test_debug_functionality
    
    # 显示测试结果
    show_test_results
}

# 执行主函数
main "$@"