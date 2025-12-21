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

# 脚本目录和配置
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
OPENYURT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
KIND_CLUSTER_NAME="${KIND_CLUSTER_NAME:-tenantcluster-test}"
NAMESPACE="${NAMESPACE:-openyurt-system}"
TEST_TIMEOUT="${TEST_TIMEOUT:-600}"  # 10 minutes
CLEANUP_ON_EXIT="${CLEANUP_ON_EXIT:-true}"
REPORT_DIR="${REPORT_DIR:-./test-reports}"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
REPORT_FILE="${REPORT_DIR}/tenantcluster-test-report-${TIMESTAMP}.txt"

# 测试结果统计
TESTS_TOTAL=0
TESTS_PASSED=0
TESTS_FAILED=0
TESTS_SKIPPED=0

# 测试资源名称前缀
TEST_PREFIX="tc-test-$(date +%s)"

# 日志函数
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1" | tee -a "$REPORT_FILE"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1" | tee -a "$REPORT_FILE"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1" | tee -a "$REPORT_FILE"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1" | tee -a "$REPORT_FILE"
}

log_test() {
    echo -e "${BLUE}[TEST]${NC} $1" | tee -a "$REPORT_FILE"
}

# 显示帮助信息
show_help() {
    cat << EOF
用法: $0 [选项]

TenantCluster 功能测试脚本

选项:
    -h, --help                          显示此帮助信息
    -c, --cluster <name>                Kind 集群名称 (默认: tenantcluster-test)
    -n, --namespace <namespace>         命名空间 (默认: openyurt-system)
    -t, --timeout <seconds>             测试超时时间 (默认: 600)
    --no-cleanup                        测试完成后不清理资源
    --report-dir <dir>                  报告目录 (默认: ./test-reports)
    --scenario <scenario>               运行特定测试场景
    --skip-deployment                   跳过 operator 部署检查

测试场景:
    basic                               基本功能测试
    custom-config                       自定义配置测试
    error-handling                      错误处理测试
    update                              更新场景测试
    network                             网络连通性测试
    all                                所有测试 (默认)

示例:
    $0                                  # 运行所有测试
    $0 --scenario basic                 # 仅运行基本功能测试
    $0 --no-cleanup --timeout 1200      # 不清理资源，超时时间 20 分钟
EOF
}

# 解析命令行参数
parse_args() {
    local scenario=""
    
    while [[ $# -gt 0 ]]; do
        case $1 in
            -h|--help)
                show_help
                exit 0
                ;;
            -c|--cluster)
                KIND_CLUSTER_NAME="$2"
                shift 2
                ;;
            -n|--namespace)
                NAMESPACE="$2"
                shift 2
                ;;
            -t|--timeout)
                TEST_TIMEOUT="$2"
                shift 2
                ;;
            --no-cleanup)
                CLEANUP_ON_EXIT="false"
                shift
                ;;
            --report-dir)
                REPORT_DIR="$2"
                REPORT_FILE="${REPORT_DIR}/tenantcluster-test-report-${TIMESTAMP}.txt"
                shift 2
                ;;
            --scenario)
                scenario="$2"
                shift 2
                ;;
            --skip-deployment)
                SKIP_DEPLOYMENT_CHECK="true"
                shift
                ;;
            *)
                log_error "未知参数: $1"
                show_help
                exit 1
                ;;
        esac
    done
    
    # 设置测试场景
    if [ -n "$scenario" ]; then
        case $scenario in
            basic|custom-config|error-handling|update|network|all)
                TEST_SCENARIOS=("$scenario")
                ;;
            *)
                log_error "无效的测试场景: $scenario"
                exit 1
                ;;
        esac
    else
        TEST_SCENARIOS=(all)
    fi
}

# 测试函数
test_passed() {
    log_success "$1"
    ((TESTS_PASSED++))
    ((TESTS_TOTAL++))
}

test_failed() {
    log_error "$1"
    ((TESTS_FAILED++))
    ((TESTS_TOTAL++))
}

test_skipped() {
    log_warning "$1"
    ((TESTS_SKIPPED++))
    ((TESTS_TOTAL++))
}

# 初始化测试环境
init_test_environment() {
    log_info "初始化测试环境..."
    
    # 创建报告目录
    mkdir -p "$REPORT_DIR"
    
    # 初始化报告文件
    cat > "$REPORT_FILE" <<EOF
=======================================
TenantCluster 功能测试报告
=======================================
开始时间: $(date)
集群名称: $KIND_CLUSTER_NAME
命名空间: $NAMESPACE
测试超时: ${TEST_TIMEOUT}秒
=======================================

EOF
    
    # 检查依赖
    if ! command -v kubectl &> /dev/null; then
        test_failed "kubectl 未安装"
        exit 1
    fi
    
    if ! command -v docker &> /dev/null; then
        test_failed "docker 未安装"
        exit 1
    fi
    
    if ! command -v kind &> /dev/null; then
        test_failed "kind 未安装"
        exit 1
    fi
    
    # 检查 Kind 集群
    if ! kind get clusters | grep -q "^${KIND_CLUSTER_NAME}$"; then
        test_failed "Kind 集群 '${KIND_CLUSTER_NAME}' 不存在"
        exit 1
    fi
    
    # 设置 kubectl 上下文
    kubectl config use-context "kind-${KIND_CLUSTER_NAME}"
    
    # 检查 TenantCluster CRD
    if ! kubectl get crd tenantclusters.apps.openyurt.io &> /dev/null; then
        test_failed "TenantCluster CRD 未安装"
        exit 1
    fi
    
    # 检查 operator pod
    if [ "${SKIP_DEPLOYMENT_CHECK}" != "true" ]; then
        local pod_status=$(kubectl get pods -n "${NAMESPACE}" -l app=yurt-manager -o jsonpath='{.items[0].status.phase}' 2>/dev/null || echo "")
        if [ "${pod_status}" != "Running" ]; then
            test_failed "yurt-manager pod 未运行: ${pod_status}"
            exit 1
        fi
    fi
    
    # 检查 CA secret
    if ! kubectl get secret host-k8s-ca -n kube-system &> /dev/null; then
        test_failed "host-k8s-ca secret 不存在"
        exit 1
    fi
    
    test_passed "测试环境初始化完成"
}

# 等待资源就绪
wait_for_resource() {
    local resource_type=$1
    local resource_name=$2
    local namespace=$3
    local condition=$4
    local timeout=${5:-300}
    local interval=${6:-5}
    
    local elapsed=0
    while [ $elapsed -lt $timeout ]; do
        local status
        case $resource_type in
            tenantcluster)
                status=$(kubectl get tenantcluster "$resource_name" -o jsonpath="{$condition}" 2>/dev/null || echo "")
                ;;
            pod)
                status=$(kubectl get pod "$resource_name" -n "$namespace" -o jsonpath="{$condition}" 2>/dev/null || echo "")
                ;;
            service)
                status=$(kubectl get service "$resource_name" -n "$namespace" -o jsonpath="{$condition}" 2>/dev/null || echo "")
                ;;
            *)
                status=$(kubectl get "$resource_type" "$resource_name" -n "$namespace" -o jsonpath="{$condition}" 2>/dev/null || echo "")
                ;;
        esac
        
        if [ -n "$status" ] && [ "$status" != "False" ]; then
            return 0
        fi
        
        sleep $interval
        elapsed=$((elapsed + interval))
    done
    
    return 1
}

# 创建基本 TenantCluster
create_basic_tenantcluster() {
    local name=$1
    local temp_file="/tmp/${name}.yaml"
    
    cat > "$temp_file" <<EOF
apiVersion: apps.openyurt.io/v1alpha1
kind: TenantCluster
metadata:
  name: $name
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
    
    kubectl apply -f "$temp_file"
    rm -f "$temp_file"
}

# 创建自定义配置 TenantCluster
create_custom_tenantcluster() {
    local name=$1
    local temp_file="/tmp/${name}.yaml"
    
    cat > "$temp_file" <<EOF
apiVersion: apps.openyurt.io/v1alpha1
kind: TenantCluster
metadata:
  name: $name
spec:
  kubernetesVersion: "v1.32.0"
  apiServerServiceIP: "10.96.100.2"
  apiServerNodePort: 31001
  hostControlPlaneAddr: "172.18.0.2:6443"
  hostCASecretRef:
    name: host-k8s-ca
    namespace: kube-system
  etcdReplicas: 1
  controllerManagerReplicas: 1
  schedulerReplicas: 1
  etcdStorageClass: "local-path"
  etcdStorageSize: "2Gi"
  imageRepository: "registry.aliyuncs.com/google_containers"
  etcdImageRepository: "quay.io/coreos"
  etcdVersion: "v3.6.0"
  serviceClusterIPRange: "10.96.0.0/12"
  podCIDR: "10.244.0.0/16"
EOF
    
    kubectl apply -f "$temp_file"
    rm -f "$temp_file"
}

# 创建无效配置 TenantCluster
create_invalid_tenantcluster() {
    local name=$1
    local temp_file="/tmp/${name}.yaml"
    
    cat > "$temp_file" <<EOF
apiVersion: apps.openyurt.io/v1alpha1
kind: TenantCluster
metadata:
  name: $name
spec:
  kubernetesVersion: "v1.32.0"
  apiServerServiceIP: "10.96.100.3"
  apiServerNodePort: 31002
  hostControlPlaneAddr: "172.18.0.2:6443"
  hostCASecretRef:
    name: nonexistent-secret
    namespace: kube-system
  etcdReplicas: 10  # 超出最大值
  etcdStorageClass: "local-path"
  etcdStorageSize: "1Gi"
EOF
    
    kubectl apply -f "$temp_file" 2>/dev/null || true
    rm -f "$temp_file"
}

# 基本功能测试
test_basic_functionality() {
    log_test "开始基本功能测试..."
    
    local tc_name="${TEST_PREFIX}-basic"
    
    # 测试创建 TenantCluster
    log_info "创建基本 TenantCluster: $tc_name"
    create_basic_tenantcluster "$tc_name"
    
    if ! kubectl get tenantcluster "$tc_name" &> /dev/null; then
        test_failed "TenantCluster 创建失败"
        return 1
    fi
    
    # 等待 TenantCluster 处理
    log_info "等待 TenantCluster 初始化..."
    if ! wait_for_resource "tenantcluster" "$tc_name" "" ".status.phase" 300; then
        test_failed "TenantCluster 初始化超时"
        kubectl delete tenantcluster "$tc_name" --ignore-not-found=true
        return 1
    fi
    
    # 检查命名空间创建
    local tc_namespace=$(kubectl get tenantcluster "$tc_name" -o jsonpath='{.status.namespace}' 2>/dev/null || echo "")
    if [ -z "$tc_namespace" ]; then
        test_failed "TenantCluster 命名空间未创建"
        kubectl delete tenantcluster "$tc_name" --ignore-not-found=true
        return 1
    fi
    
    if ! kubectl get namespace "$tc_namespace" &> /dev/null; then
        test_failed "TenantCluster 命名空间不存在: $tc_namespace"
        kubectl delete tenantcluster "$tc_name" --ignore-not-found=true
        return 1
    fi
    
    # 检查控制平面组件
    log_info "检查控制平面组件..."
    
    # 检查 etcd
    local etcd_replicas=$(kubectl get tenantcluster "$tc_name" -o jsonpath='{.spec.etcdReplicas}' 2>/dev/null || echo "3")
    if ! kubectl get statefulset -n "$tc_namespace" -l app=etcd &> /dev/null; then
        test_failed "etcd StatefulSet 未创建"
        kubectl delete tenantcluster "$tc_name" --ignore-not-found=true
        return 1
    fi
    
    # 检查 API Server
    if ! kubectl get deployment -n "$tc_namespace" -l app=kube-apiserver &> /dev/null; then
        test_failed "API Server Deployment 未创建"
        kubectl delete tenantcluster "$tc_name" --ignore-not-found=true
        return 1
    fi
    
    # 检查 Controller Manager
    if ! kubectl get deployment -n "$tc_namespace" -l app=kube-controller-manager &> /dev/null; then
        test_failed "Controller Manager Deployment 未创建"
        kubectl delete tenantcluster "$tc_name" --ignore-not-found=true
        return 1
    fi
    
    # 检查 Scheduler
    if ! kubectl get deployment -n "$tc_namespace" -l app=kube-scheduler &> /dev/null; then
        test_failed "Scheduler Deployment 未创建"
        kubectl delete tenantcluster "$tc_name" --ignore-not-found=true
        return 1
    fi
    
    # 检查服务
    if ! kubectl get service -n "$tc_namespace" tenant-apiserver &> /dev/null; then
        test_failed "API Server Service 未创建"
        kubectl delete tenantcluster "$tc_name" --ignore-not-found=true
        return 1
    fi
    
    # 检查 kubeconfig secret
    local kubeconfig_secret=$(kubectl get tenantcluster "$tc_name" -o jsonpath='{.status.adminKubeconfigSecret}' 2>/dev/null || echo "")
    if [ -z "$kubeconfig_secret" ]; then
        test_failed "kubeconfig secret 未生成"
        kubectl delete tenantcluster "$tc_name" --ignore-not-found=true
        return 1
    fi
    
    if ! kubectl get secret -n "$tc_namespace" "$kubeconfig_secret" &> /dev/null; then
        test_failed "kubeconfig secret 不存在: $kubeconfig_secret"
        kubectl delete tenantcluster "$tc_name" --ignore-not-found=true
        return 1
    fi
    
    # 清理
    kubectl delete tenantcluster "$tc_name" --ignore-not-found=true
    
    test_passed "基本功能测试通过"
}

# 自定义配置测试
test_custom_configuration() {
    log_test "开始自定义配置测试..."
    
    local tc_name="${TEST_PREFIX}-custom"
    
    # 创建自定义配置的 TenantCluster
    log_info "创建自定义配置 TenantCluster: $tc_name"
    create_custom_tenantcluster "$tc_name"
    
    if ! kubectl get tenantcluster "$tc_name" &> /dev/null; then
        test_failed "自定义 TenantCluster 创建失败"
        return 1
    fi
    
    # 验证自定义配置
    log_info "验证自定义配置..."
    
    # 检查 etcd 副本数
    local etcd_replicas=$(kubectl get tenantcluster "$tc_name" -o jsonpath='{.spec.etcdReplicas}' 2>/dev/null || echo "")
    if [ "$etcd_replicas" != "1" ]; then
        test_failed "etcd 副本数配置错误: 期望 1，实际 $etcd_replicas"
        kubectl delete tenantcluster "$tc_name" --ignore-not-found=true
        return 1
    fi
    
    # 检查 controller-manager 副本数
    local cm_replicas=$(kubectl get tenantcluster "$tc_name" -o jsonpath='{.spec.controllerManagerReplicas}' 2>/dev/null || echo "")
    if [ "$cm_replicas" != "1" ]; then
        test_failed "controller-manager 副本数配置错误: 期望 1，实际 $cm_replicas"
        kubectl delete tenantcluster "$tc_name" --ignore-not-found=true
        return 1
    fi
    
    # 检查 scheduler 副本数
    local scheduler_replicas=$(kubectl get tenantcluster "$tc_name" -o jsonpath='{.spec.schedulerReplicas}' 2>/dev/null || echo "")
    if [ "$scheduler_replicas" != "1" ]; then
        test_failed "scheduler 副本数配置错误: 期望 1，实际 $scheduler_replicas"
        kubectl delete tenantcluster "$tc_name" --ignore-not-found=true
        return 1
    fi
    
    # 检查存储大小
    local storage_size=$(kubectl get tenantcluster "$tc_name" -o jsonpath='{.spec.etcdStorageSize}' 2>/dev/null || echo "")
    if [ "$storage_size" != "2Gi" ]; then
        test_failed "etcd 存储大小配置错误: 期望 2Gi，实际 $storage_size"
        kubectl delete tenantcluster "$tc_name" --ignore-not-found=true
        return 1
    fi
    
    # 等待资源创建
    local tc_namespace=$(kubectl get tenantcluster "$tc_name" -o jsonpath='{.status.namespace}' 2>/dev/null || echo "")
    if [ -n "$tc_namespace" ]; then
        # 检查 PVC 大小
        log_info "检查 PVC 配置..."
        local pvc_size=$(kubectl get pvc -n "$tc_namespace" -l app=etcd -o jsonpath='{.items[0].spec.resources.requests.storage}' 2>/dev/null || echo "")
        if [ "$pvc_size" != "2Gi" ]; then
            test_failed "PVC 存储大小错误: 期望 2Gi，实际 $pvc_size"
            kubectl delete tenantcluster "$tc_name" --ignore-not-found=true
            return 1
        fi
    fi
    
    # 清理
    kubectl delete tenantcluster "$tc_name" --ignore-not-found=true
    
    test_passed "自定义配置测试通过"
}

# 错误处理测试
test_error_handling() {
    log_test "开始错误处理测试..."
    
    local tc_name="${TEST_PREFIX}-error"
    
    # 测试无效配置
    log_info "测试无效配置处理..."
    create_invalid_tenantcluster "$tc_name"
    
    # 检查是否正确拒绝无效配置
    if kubectl get tenantcluster "$tc_name" &> /dev/null; then
        # 如果创建了，检查是否有错误条件
        local phase=$(kubectl get tenantcluster "$tc_name" -o jsonpath='{.status.phase}' 2>/dev/null || echo "")
        local conditions=$(kubectl get tenantcluster "$tc_name" -o jsonpath='{.status.conditions[*].type}' 2>/dev/null || echo "")
        
        if echo "$conditions" | grep -q "Failed"; then
            test_passed "正确检测到无效配置"
        else
            test_failed "未正确检测到无效配置"
            kubectl delete tenantcluster "$tc_name" --ignore-not-found=true
            return 1
        fi
    else
        test_passed "正确拒绝无效配置"
    fi
    
    # 清理
    kubectl delete tenantcluster "$tc_name" --ignore-not-found=true
    
    # 测试不存在的 CA secret
    log_info "测试不存在的 CA secret..."
    tc_name="${TEST_PREFIX}-ca-error"
    
    cat > "/tmp/${tc_name}.yaml" <<EOF
apiVersion: apps.openyurt.io/v1alpha1
kind: TenantCluster
metadata:
  name: $tc_name
spec:
  kubernetesVersion: "v1.32.0"
  apiServerServiceIP: "10.96.100.4"
  apiServerNodePort: 31003
  hostControlPlaneAddr: "172.18.0.2:6443"
  hostCASecretRef:
    name: nonexistent-ca-secret
    namespace: kube-system
  etcdStorageClass: "local-path"
  etcdStorageSize: "1Gi"
EOF
    
    if kubectl apply -f "/tmp/${tc_name}.yaml" 2>/dev/null; then
        # 等待处理
        sleep 10
        
        # 检查是否有错误条件
        local conditions=$(kubectl get tenantcluster "$tc_name" -o jsonpath='{.status.conditions[*].type}' 2>/dev/null || echo "")
        if echo "$conditions" | grep -q "Failed"; then
            test_passed "正确检测到 CA secret 不存在"
        else
            test_failed "未正确检测到 CA secret 不存在"
        fi
        
        kubectl delete tenantcluster "$tc_name" --ignore-not-found=true
    else
        test_passed "正确拒绝不存在的 CA secret"
    fi
    
    rm -f "/tmp/${tc_name}.yaml"
    
    test_passed "错误处理测试通过"
}

# 更新场景测试
test_update_scenario() {
    log_test "开始更新场景测试..."
    
    local tc_name="${TEST_PREFIX}-update"
    
    # 创建基本 TenantCluster
    log_info "创建 TenantCluster 用于更新测试: $tc_name"
    create_basic_tenantcluster "$tc_name"
    
    if ! kubectl get tenantcluster "$tc_name" &> /dev/null; then
        test_failed "TenantCluster 创建失败"
        return 1
    fi
    
    # 等待初始化
    if ! wait_for_resource "tenantcluster" "$tc_name" "" ".status.phase" 300; then
        test_failed "TenantCluster 初始化超时"
        kubectl delete tenantcluster "$tc_name" --ignore-not-found=true
        return 1
    fi
    
    # 获取初始配置
    local initial_etcd_replicas=$(kubectl get tenantcluster "$tc_name" -o jsonpath='{.spec.etcdReplicas}' 2>/dev/null || echo "3")
    local initial_storage_size=$(kubectl get tenantcluster "$tc_name" -o jsonpath='{.spec.etcdStorageSize}' 2>/dev/null || echo "1Gi")
    
    # 更新配置
    log_info "更新 TenantCluster 配置..."
    kubectl patch tenantcluster "$tc_name" --type='merge' -p='{
        "spec": {
            "etcdReplicas": 1,
            "etcdStorageSize": "2Gi"
        }
    }'
    
    # 等待更新生效
    sleep 10
    
    # 验证更新
    local updated_etcd_replicas=$(kubectl get tenantcluster "$tc_name" -o jsonpath='{.spec.etcdReplicas}' 2>/dev/null || echo "")
    local updated_storage_size=$(kubectl get tenantcluster "$tc_name" -o jsonpath='{.spec.etcdStorageSize}' 2>/dev/null || echo "")
    
    if [ "$updated_etcd_replicas" != "1" ]; then
        test_failed "etcd 副本数更新失败: 期望 1，实际 $updated_etcd_replicas"
        kubectl delete tenantcluster "$tc_name" --ignore-not-found=true
        return 1
    fi
    
    if [ "$updated_storage_size" != "2Gi" ]; then
        test_failed "etcd 存储大小更新失败: 期望 2Gi，实际 $updated_storage_size"
        kubectl delete tenantcluster "$tc_name" --ignore-not-found=true
        return 1
    fi
    
    # 测试暂停和恢复
    log_info "测试暂停功能..."
    kubectl patch tenantcluster "$tc_name" --type='merge' -p='{"spec": {"paused": true}}'
    
    sleep 5
    
    local paused=$(kubectl get tenantcluster "$tc_name" -o jsonpath='{.spec.paused}' 2>/dev/null || echo "")
    if [ "$paused" != "true" ]; then
        test_failed "暂停功能失败"
        kubectl delete tenantcluster "$tc_name" --ignore-not-found=true
        return 1
    fi
    
    # 恢复
    kubectl patch tenantcluster "$tc_name" --type='merge' -p='{"spec": {"paused": false}}'
    
    sleep 5
    
    paused=$(kubectl get tenantcluster "$tc_name" -o jsonpath='{.spec.paused}' 2>/dev/null || echo "")
    if [ "$paused" != "false" ]; then
        test_failed "恢复功能失败"
        kubectl delete tenantcluster "$tc_name" --ignore-not-found=true
        return 1
    fi
    
    # 清理
    kubectl delete tenantcluster "$tc_name" --ignore-not-found=true
    
    test_passed "更新场景测试通过"
}

# 网络连通性测试
test_network_connectivity() {
    log_test "开始网络连通性测试..."
    
    local tc_name="${TEST_PREFIX}-network"
    
    # 创建 TenantCluster
    log_info "创建 TenantCluster 用于网络测试: $tc_name"
    create_basic_tenantcluster "$tc_name"
    
    if ! kubectl get tenantcluster "$tc_name" &> /dev/null; then
        test_failed "TenantCluster 创建失败"
        return 1
    fi
    
    # 等待初始化
    if ! wait_for_resource "tenantcluster" "$tc_name" "" ".status.phase" 300; then
        test_failed "TenantCluster 初始化超时"
        kubectl delete tenantcluster "$tc_name" --ignore-not-found=true
        return 1
    fi
    
    # 获取命名空间和服务信息
    local tc_namespace=$(kubectl get tenantcluster "$tc_name" -o jsonpath='{.status.namespace}' 2>/dev/null || echo "")
    if [ -z "$tc_namespace" ]; then
        test_failed "无法获取 TenantCluster 命名空间"
        kubectl delete tenantcluster "$tc_name" --ignore-not-found=true
        return 1
    fi
    
    # 等待 API Server 服务就绪
    log_info "等待 API Server 服务就绪..."
    if ! wait_for_resource "service" "tenant-apiserver" "$tc_namespace" ".spec.clusterIP" 300; then
        test_failed "API Server 服务未就绪"
        kubectl delete tenantcluster "$tc_name" --ignore-not-found=true
        return 1
    fi
    
    # 获取服务 IP
    local api_server_ip=$(kubectl get service tenant-apiserver -n "$tc_namespace" -o jsonpath='{.spec.clusterIP}' 2>/dev/null || echo "")
    if [ -z "$api_server_ip" ]; then
        test_failed "无法获取 API Server 服务 IP"
        kubectl delete tenantcluster "$tc_name" --ignore-not-found=true
        return 1
    fi
    
    # 测试内部连通性
    log_info "测试内部网络连通性..."
    # 创建一个测试 pod
    cat > "/tmp/test-pod.yaml" <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: network-test
  namespace: $tc_namespace
spec:
  containers:
  - name: test
    image: busybox:1.35
    command: ['sh', '-c', 'sleep 3600']
EOF
    
    kubectl apply -f "/tmp/test-pod.yaml"
    rm -f "/tmp/test-pod.yaml"
    
    # 等待测试 pod 就绪
    if ! wait_for_resource "pod" "network-test" "$tc_namespace" ".status.phase" 120; then
        test_failed "测试 pod 未就绪"
        kubectl delete tenantcluster "$tc_name" --ignore-not-found=true
        return 1
    fi
    
    # 测试连接 API Server
    log_info "测试连接 API Server..."
    local test_result=$(kubectl exec -n "$tc_namespace" network-test -- wget -q --spider --timeout=5 "https://$api_server_ip:443/healthz" 2>/dev/null || echo "failed")
    if [ "$test_result" = "failed" ]; then
        test_failed "无法连接到 API Server"
        kubectl delete pod network-test -n "$tc_namespace" --ignore-not-found=true
        kubectl delete tenantcluster "$tc_name" --ignore-not-found=true
        return 1
    fi
    
    # 清理测试 pod
    kubectl delete pod network-test -n "$tc_namespace" --ignore-not-found=true
    
    # 测试外部访问
    log_info "测试外部访问..."
    local nodeport=$(kubectl get tenantcluster "$tc_name" -o jsonpath='{.spec.apiServerNodePort}' 2>/dev/null || echo "")
    if [ -n "$nodeport" ]; then
        # 获取节点 IP
        local node_ip=$(kubectl get nodes -o jsonpath='{.items[0].status.addresses[?(@.type=="InternalIP")].address}' 2>/dev/null || echo "")
        if [ -n "$node_ip" ]; then
            # 从主机测试连接
            if command -v curl &> /dev/null; then
                local curl_result=$(curl -k --connect-timeout 5 --max-time 10 "https://$node_ip:$nodeport/healthz" 2>/dev/null || echo "failed")
                if [ "$curl_result" != "failed" ]; then
                    test_passed "外部网络连通性测试通过"
                else
                    test_warning "外部网络连通性测试失败，可能是网络配置问题"
                fi
            else
                test_warning "curl 未安装，跳过外部网络连通性测试"
            fi
        else
            test_warning "无法获取节点 IP，跳过外部网络连通性测试"
        fi
    else
        test_warning "无法获取 NodePort，跳过外部网络连通性测试"
    fi
    
    # 清理
    kubectl delete tenantcluster "$tc_name" --ignore-not-found=true
    
    test_passed "网络连通性测试通过"
}

# 清理测试资源
cleanup_test_resources() {
    if [ "$CLEANUP_ON_EXIT" != "true" ]; then
        log_info "跳过资源清理 (CLEANUP_ON_EXIT=false)"
        return
    fi
    
    log_info "清理测试资源..."
    
    # 删除所有测试 TenantCluster
    local test_clusters=$(kubectl get tenantcluster --no-headers | grep "^${TEST_PREFIX}" | awk '{print $1}' || echo "")
    if [ -n "$test_clusters" ]; then
        echo "$test_clusters" | xargs -I {} kubectl delete tenantcluster {} --ignore-not-found=true || true
    fi
    
    # 删除测试命名空间
    local test_namespaces=$(kubectl get namespaces --no-headers | grep "^${TEST_PREFIX}" | awk '{print $1}' || echo "")
    if [ -n "$test_namespaces" ]; then
        echo "$test_namespaces" | xargs -I {} kubectl delete namespace {} --ignore-not-found=true || true
    fi
    
    # 删除临时文件
    rm -f /tmp/${TEST_PREFIX}*.yaml 2>/dev/null || true
    
    log_info "测试资源清理完成"
}

# 生成测试报告
generate_test_report() {
    log_info "生成测试报告..."
    
    cat >> "$REPORT_FILE" <<EOF

=======================================
测试结果汇总
=======================================
开始时间: $(date)
结束时间: $(date)
总测试数: $TESTS_TOTAL
通过: $TESTS_PASSED
失败: $TESTS_FAILED
跳过: $TESTS_SKIPPED
成功率: $(( TESTS_TOTAL > 0 ? (TESTS_PASSED * 100) / TESTS_TOTAL : 0 ))%

=======================================
详细测试结果
=======================================
EOF
    
    # 添加测试场景信息
    for scenario in "${TEST_SCENARIOS[@]}"; do
        echo "测试场景: $scenario" >> "$REPORT_FILE"
    done
    
    echo "" >> "$REPORT_FILE"
    
    # 添加失败的测试详情
    if [ $TESTS_FAILED -gt 0 ]; then
        echo "失败的测试:" >> "$REPORT_FILE"
        # 这里可以添加更详细的失败信息
    fi
    
    # 添加系统信息
    cat >> "$REPORT_FILE" <<EOF

=======================================
系统信息
=======================================
Kubernetes 版本:
$(kubectl version --short=true 2>/dev/null || echo "无法获取")

Kind 集群: $KIND_CLUSTER_NAME
命名空间: $NAMESPACE
测试超时: ${TEST_TIMEOUT}秒

=======================================
资源状态
=======================================
TenantCluster CRD:
$(kubectl get crd tenantclusters.apps.openyurt.io -o yaml 2>/dev/null || echo "未找到")

Operator Pods:
$(kubectl get pods -n "$NAMESPACE" -l app=yurt-manager 2>/dev/null || echo "未找到")

现有 TenantClusters:
$(kubectl get tenantcluster 2>/dev/null || echo "无")

EOF
    
    log_success "测试报告已生成: $REPORT_FILE"
}

# 显示测试结果
show_test_results() {
    echo ""
    echo "======================================"
    echo "测试结果汇总"
    echo "======================================"
    echo "总测试数: $TESTS_TOTAL"
    echo "通过: $TESTS_PASSED"
    echo "失败: $TESTS_FAILED"
    echo "跳过: $TESTS_SKIPPED"
    echo "成功率: $(( TESTS_TOTAL > 0 ? (TESTS_PASSED * 100) / TESTS_TOTAL : 0 ))%"
    echo ""
    echo "详细报告: $REPORT_FILE"
    
    if [ $TESTS_FAILED -eq 0 ]; then
        log_success "所有测试通过!"
        return 0
    else
        log_error "有 $TESTS_FAILED 个测试失败"
        return 1
    fi
}

# 主函数
main() {
    log_info "开始 TenantCluster 功能测试..."
    
    # 解析参数
    parse_args "$@"
    
    # 设置陷阱以清理资源
    trap cleanup_test_resources EXIT
    
    # 初始化测试环境
    init_test_environment
    
    # 运行测试场景
    for scenario in "${TEST_SCENARIOS[@]}"; do
        case $scenario in
            basic)
                test_basic_functionality
                ;;
            custom-config)
                test_custom_configuration
                ;;
            error-handling)
                test_error_handling
                ;;
            update)
                test_update_scenario
                ;;
            network)
                test_network_connectivity
                ;;
            all)
                test_basic_functionality
                test_custom_configuration
                test_error_handling
                test_update_scenario
                test_network_connectivity
                ;;
        esac
    done
    
    # 生成测试报告
    generate_test_report
    
    # 显示测试结果
    show_test_results
}

# 错误处理
trap 'log_error "脚本执行失败，退出码: $?"' ERR

# 执行主函数
main "$@"