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
NAMESPACE="${NAMESPACE:-openyurt-system}"
TEST_NAME="${TEST_NAME:-tc-test-$(date +%s)}"
CLEANUP="${CLEANUP:-true}"
SKIP_BASIC="${SKIP_BASIC:-false}"
SKIP_ADVANCED="${SKIP_ADVANCED:-false}"
REPORT_DIR="${REPORT_DIR:-$(pwd)/test-reports}"

# 测试结果统计
TESTS_PASSED=0
TESTS_FAILED=0
TESTS_TOTAL=0

# 显示帮助信息
show_help() {
    cat << EOF
用法: $0 [选项]

测试修复后的 TenantCluster 功能

选项:
    -h, --help                          显示此帮助信息
    -n, --name <name>                   测试名称前缀 (默认: tc-test-<timestamp>)
    --no-cleanup                        测试完成后不清理资源
    --skip-basic                        跳过基本功能测试
    --skip-advanced                     跳过高级功能测试
    --report-dir <dir>                  测试报告目录 (默认: ./test-reports)
    --cluster-name <name>               Kind 集群名称 (默认: tenantcluster-test)

示例:
    $0                                  # 运行完整测试
    $0 --skip-advanced                  # 只运行基本测试
    $0 --no-cleanup                     # 测试后保留资源
    $0 -n my-test --report-dir /tmp/reports  # 自定义测试名称和报告目录
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
            -n|--name)
                TEST_NAME="$2"
                shift 2
                ;;
            --no-cleanup)
                CLEANUP="false"
                shift
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
        exit 1
    fi
    
    log_success "依赖检查通过"
}

# 检查 Kind 集群
check_kind_cluster() {
    log_info "检查 Kind 集群: ${KIND_CLUSTER_NAME}"
    
    if ! kind get clusters | grep -q "^${KIND_CLUSTER_NAME}$"; then
        log_error "Kind 集群 '${KIND_CLUSTER_NAME}' 不存在"
        exit 1
    fi
    
    # 设置 kubectl 上下文
    kubectl config use-context "kind-${KIND_CLUSTER_NAME}"
    
    log_success "Kind 集群检查通过"
}

# 创建报告目录
setup_report_dir() {
    mkdir -p "${REPORT_DIR}"
    REPORT_FILE="${REPORT_DIR}/tenantcluster-test-$(date +%Y%m%d-%H%M%S).md"
    
    cat > "${REPORT_FILE}" << EOF
# TenantCluster 测试报告

## 测试概述

本报告记录了对修复后的 OpenYurt TenantCluster 功能的测试结果。

## 测试环境

- **集群类型**: Kind
- **集群名称**: ${KIND_CLUSTER_NAME}
- **Kubernetes 版本**: $(kubectl version --short | grep 'Server Version' | cut -d' ' -f3)
- **测试名称**: ${TEST_NAME}
- **测试时间**: $(date -u +"%Y-%m-%dT%H:%M:%SZ")

## 测试步骤和结果

EOF
    
    log_info "测试报告将保存到: ${REPORT_FILE}"
}

# 更新报告
update_report() {
    local test_name="$1"
    local status="$2"
    local details="$3"
    
    local status_icon="❌"
    if [ "$status" = "PASSED" ]; then
        status_icon="✅"
    elif [ "$status" = "WARNING" ]; then
        status_icon="⚠️"
    fi
    
    cat >> "${REPORT_FILE}" << EOF
### ${test_name} ${status_icon}

**结果**: ${status}

**详情**:
${details}

EOF
}

# 测试函数
run_test() {
    local test_name="$1"
    local test_command="$2"
    local expected_result="${3:-0}"
    
    log_info "运行测试: ${test_name}"
    ((TESTS_TOTAL++))
    
    if eval "$test_command" > /dev/null 2>&1; then
        log_success "测试通过: ${test_name}"
        ((TESTS_PASSED++))
        update_report "$test_name" "PASSED" "命令执行成功: \`\`\`bash\n${test_command}\n\`\`\`"
        return 0
    else
        log_error "测试失败: ${test_name}"
        ((TESTS_FAILED++))
        update_report "$test_name" "FAILED" "命令执行失败: \`\`\`bash\n${test_command}\n\`\`\`"
        return 1
    fi
}

# 测试 operator 状态
test_operator_status() {
    log_info "测试 operator 状态..."
    
    # 检查 yurt-manager pod 是否运行
    local pod_status=$(kubectl get pods -n "${NAMESPACE}" -l app=yurt-manager -o jsonpath='{.items[0].status.phase}' 2>/dev/null || echo "")
    if [ "$pod_status" != "Running" ]; then
        log_error "yurt-manager pod 未运行，状态: $pod_status"
        update_report "Operator 状态检查" "FAILED" "yurt-manager pod 状态: $pod_status"
        return 1
    fi
    
    # 检查 TenantCluster controller 是否启动
    local controller_logs=$(kubectl logs -n "${NAMESPACE}" -l app=yurt-manager --tail=20 2>/dev/null || echo "")
    if ! echo "${controller_logs}" | grep -q "tenantcluster-controller"; then
        log_warning "TenantCluster controller 未在日志中找到"
        update_report "TenantCluster Controller 检查" "WARNING" "TenantCluster controller 未在日志中找到，可能仍在启动中"
    else
        log_success "TenantCluster controller 已启动"
        update_report "TenantCluster Controller 检查" "PASSED" "TenantCluster controller 已在日志中找到"
    fi
    
    log_success "operator 状态检查完成"
    return 0
}

# 创建测试用的 TenantCluster
create_test_tenantcluster() {
    log_info "创建测试用的 TenantCluster..."
    
    # 创建测试命名空间
    kubectl create namespace "${TEST_NAME}" || true
    
    # 创建 TenantCluster
    cat << EOF | kubectl apply -f -
apiVersion: apps.openyurt.io/v1alpha1
kind: TenantCluster
metadata:
  name: ${TEST_NAME}
  namespace: ${TEST_NAME}
spec:
  version: "v1.28.0"
  replicas: 1
  imageRepository: "registry.k8s.io"
EOF
    
    # 等待 TenantCluster 创建完成
    local max_wait=300
    local wait_time=0
    while [ $wait_time -lt $max_wait ]; do
        local phase=$(kubectl get tenantcluster "${TEST_NAME}" -n "${TEST_NAME}" -o jsonpath='{.status.phase}' 2>/dev/null || echo "")
        if [ "$phase" = "Running" ]; then
            log_success "TenantCluster 已创建并运行"
            update_report "TenantCluster 创建" "PASSED" "TenantCluster ${TEST_NAME} 已创建并运行"
            return 0
        elif [ "$phase" = "Failed" ]; then
            log_error "TenantCluster 创建失败"
            update_report "TenantCluster 创建" "FAILED" "TenantCluster ${TEST_NAME} 创建失败"
            return 1
        fi
        
        sleep 5
        ((wait_time+=5))
        log_info "等待 TenantCluster 创建... (${wait_time}s/${max_wait}s)"
    done
    
    log_error "TenantCluster 创建超时"
    update_report "TenantCluster 创建" "FAILED" "TenantCluster ${TEST_NAME} 创建超时"
    return 1
}

# 测试基本功能
test_basic_functionality() {
    if [ "$SKIP_BASIC" = "true" ]; then
        log_info "跳过基本功能测试"
        return 0
    fi
    
    log_info "开始基本功能测试..."
    
    # 测试 TenantCluster 资源
    run_test "TenantCluster 资源获取" "kubectl get tenantcluster ${TEST_NAME} -n ${TEST_NAME}"
    
    # 测试控制平面组件
    run_test "API Server Deployment" "kubectl get deployment -n ${TEST_NAME} ${TEST_NAME}-apiserver"
    run_test "Controller Manager Deployment" "kubectl get deployment -n ${TEST_NAME} ${TEST_NAME}-controller-manager"
    run_test "Scheduler Deployment" "kubectl get deployment -n ${TEST_NAME} ${TEST_NAME}-scheduler"
    run_test "ETCD StatefulSet" "kubectl get statefulset -n ${TEST_NAME} ${TEST_NAME}-etcd"
    
    # 测试服务
    run_test "API Server Service" "kubectl get service -n ${TEST_NAME} ${TEST_NAME}-apiserver"
    
    # 测试 pod 状态
    run_test "API Server Pod 运行" "kubectl get pods -n ${TEST_NAME} -l component=apiserver -o jsonpath='{.items[0].status.phase}' | grep -q Running"
    run_test "Controller Manager Pod 运行" "kubectl get pods -n ${TEST_NAME} -l component=controller-manager -o jsonpath='{.items[0].status.phase}' | grep -q Running"
    run_test "Scheduler Pod 运行" "kubectl get pods -n ${TEST_NAME} -l component=scheduler -o jsonpath='{.items[0].status.phase}' | grep -q Running"
    run_test "ETCD Pod 运行" "kubectl get pods -n ${TEST_NAME} -l component=etcd -o jsonpath='{.items[0].status.phase}' | grep -q Running"
    
    log_success "基本功能测试完成"
}

# 测试高级功能
test_advanced_functionality() {
    if [ "$SKIP_ADVANCED" = "true" ]; then
        log_info "跳过高级功能测试"
        return 0
    fi
    
    log_info "开始高级功能测试..."
    
    # 获取 kubeconfig
    local kubeconfig_file="${REPORT_DIR}/kubeconfig-${TEST_NAME}"
    kubectl get secret -n "${TEST_NAME}" "${TEST_NAME}-admin-kubeconfig" -o jsonpath='{.data.kubeconfig}' | base64 -d > "${kubeconfig_file}"
    
    if [ ! -s "${kubeconfig_file}" ]; then
        log_error "无法获取 kubeconfig"
        update_report "Kubeconfig 获取" "FAILED" "无法获取 ${TEST_NAME} 的 kubeconfig"
        return 1
    fi
    
    # 设置 KUBECONFIG 环境变量
    export KUBECONFIG="${kubeconfig_file}"
    
    # 测试 tenant API Server 连接
    run_test "Tenant API Server 连接" "kubectl cluster-info"
    
    # 测试基本 kubectl 操作
    run_test "获取节点信息" "kubectl get nodes"
    run_test "获取命名空间" "kubectl get namespaces"
    
    # 创建测试命名空间
    run_test "创建测试命名空间" "kubectl create namespace test-ns"
    
    # 创建测试 pod
    cat << EOF | kubectl apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: test-pod
  namespace: test-ns
spec:
  containers:
  - name: nginx
    image: nginx:1.21-alpine
    ports:
    - containerPort: 80
EOF
    
    # 等待 pod 运行
    local max_wait=60
    local wait_time=0
    while [ $wait_time -lt $max_wait ]; do
        local pod_phase=$(kubectl get pod test-pod -n test-ns -o jsonpath='{.status.phase}' 2>/dev/null || echo "")
        if [ "$pod_phase" = "Running" ]; then
            break
        fi
        sleep 2
        ((wait_time+=2))
    done
    
    run_test "测试 Pod 运行" "kubectl get pod test-pod -n test-ns -o jsonpath='{.status.phase}' | grep -q Running"
    
    # 创建测试 service
    cat << EOF | kubectl apply -f -
apiVersion: v1
kind: Service
metadata:
  name: test-service
  namespace: test-ns
spec:
  selector:
    app: test-pod
  ports:
  - port: 80
    targetPort: 80
EOF
    
    run_test "测试 Service 创建" "kubectl get service test-service -n test-ns"
    
    # 测试 RBAC
    cat << EOF | kubectl apply -f -
apiVersion: v1
kind: ServiceAccount
metadata:
  name: test-sa
  namespace: test-ns
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: test-role
  namespace: test-ns
rules:
- apiGroups: [""]
  resources: ["pods"]
  verbs: ["get", "list", "watch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: test-rb
  namespace: test-ns
subjects:
- kind: ServiceAccount
  name: test-sa
  namespace: test-ns
roleRef:
  kind: Role
  name: test-role
  apiGroup: rbac.authorization.k8s.io
EOF
    
    run_test "RBAC 配置" "kubectl get rolebinding test-rb -n test-ns"
    
    # 清理测试资源
    kubectl delete namespace test-ns || true
    
    # 恢复 KUBECONFIG
    unset KUBECONFIG
    
    log_success "高级功能测试完成"
}

# 清理测试资源
cleanup_test_resources() {
    if [ "$CLEANUP" != "true" ]; then
        log_info "跳过资源清理"
        return
    fi
    
    log_info "清理测试资源..."
    
    # 删除 TenantCluster
    kubectl delete tenantcluster "${TEST_NAME}" -n "${TEST_NAME}" --ignore-not-found=true || true
    
    # 删除测试命名空间
    kubectl delete namespace "${TEST_NAME}" --ignore-not-found=true || true
    
    # 删除 kubeconfig 文件
    rm -f "${REPORT_DIR}/kubeconfig-${TEST_NAME}"
    
    log_success "测试资源清理完成"
}

# 生成测试报告摘要
generate_report_summary() {
    cat >> "${REPORT_FILE}" << EOF
## 测试总结

### 测试统计
- **总测试数**: ${TESTS_TOTAL}
- **通过测试**: ${TESTS_PASSED}
- **失败测试**: ${TESTS_FAILED}
- **成功率**: $(( TESTS_PASSED * 100 / TESTS_TOTAL ))%

### 性能指标
- **测试开始时间**: $(date -u +"%Y-%m-%dT%H:%M:%SZ")
- **测试结束时间**: $(date -u +"%Y-%m-%dT%H:%M:%SZ")

### 发现的问题
EOF
    
    if [ $TESTS_FAILED -eq 0 ]; then
        echo "- 无发现问题" >> "${REPORT_FILE}"
    else
        echo "- 有 ${TESTS_FAILED} 个测试失败，请查看详细日志" >> "${REPORT_FILE}"
    fi
    
    cat >> "${REPORT_FILE}" << EOF

### 建议和后续步骤
1. 如果所有测试通过，TenantCluster 功能正常
2. 如果有测试失败，请检查相关日志和配置
3. 可以继续进行更多场景的测试

---

**报告生成时间**: $(date -u +"%Y-%m-%dT%H:%M:%SZ")
**测试执行者**: 自动化测试系统
**版本**: TenantCluster v1alpha1
EOF
    
    log_success "测试报告已生成: ${REPORT_FILE}"
}

# 显示测试结果
show_test_results() {
    echo ""
    echo "======================================"
    echo "测试完成"
    echo "======================================"
    echo "总测试数: ${TESTS_TOTAL}"
    echo "通过测试: ${TESTS_PASSED}"
    echo "失败测试: ${TESTS_FAILED}"
    echo "成功率: $(( TESTS_PASSED * 100 / TESTS_TOTAL ))%"
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
    echo "  cat ${REPORT_FILE}"
}

# 主函数
main() {
    log_info "开始测试修复后的 TenantCluster 功能..."
    
    # 解析参数
    parse_args "$@"
    
    # 检查依赖
    check_dependencies
    
    # 检查 Kind 集群
    check_kind_cluster
    
    # 设置报告目录
    setup_report_dir
    
    # 测试 operator 状态
    test_operator_status
    
    # 创建测试用的 TenantCluster
    create_test_tenantcluster
    
    # 测试基本功能
    test_basic_functionality
    
    # 测试高级功能
    test_advanced_functionality
    
    # 清理测试资源
    cleanup_test_resources
    
    # 生成测试报告摘要
    generate_report_summary
    
    # 显示测试结果
    show_test_results
    
    if [ $TESTS_FAILED -eq 0 ]; then
        log_success "所有测试通过!"
        exit 0
    else
        log_error "有 ${TESTS_FAILED} 个测试失败"
        exit 1
    fi
}

# 错误处理
trap 'log_error "脚本执行失败，退出码: $?"' ERR

# 执行主函数
main "$@"