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
OUTPUT_DIR="${OUTPUT_DIR:-${SCRIPT_DIR}/debug-logs}"

# 显示帮助信息
show_help() {
    cat << EOF
用法: $0 [选项] [命令]

调试和故障排除 TenantCluster operator 的工具

命令:
    logs               获取 yurt-manager pod 日志
    status             检查部署状态
    events             显示相关事件
    describe           描述 yurt-manager pod
    tenantcluster      检查 TenantCluster 资源
    collect            收集所有调试信息
    shell              进入 yurt-manager pod shell
    clean              清理调试日志

选项:
    -h, --help                          显示此帮助信息
    -n, --namespace <namespace>         命名空间 (默认: openyurt-system)
    -o, --output <dir>                  输出目录 (默认: ./debug-logs)
    --cluster-name <name>               Kind 集群名称 (默认: openyurt)
    -f, --follow                        跟踪日志输出 (仅用于 logs 命令)

示例:
    $0 logs                              # 获取日志
    $0 logs -f                          # 跟踪日志输出
    $0 status                           # 检查状态
    $0 collect                          # 收集所有调试信息
    $0 shell                            # 进入 pod shell
EOF
}

# 解析命令行参数
parse_args() {
    COMMAND=""
    FOLLOW_LOGS=false
    
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
            -o|--output)
                OUTPUT_DIR="$2"
                shift 2
                ;;
            --cluster-name)
                KIND_CLUSTER_NAME="$2"
                shift 2
                ;;
            -f|--follow)
                FOLLOW_LOGS=true
                shift
                ;;
            logs|status|events|describe|tenantcluster|collect|shell|clean)
                COMMAND="$1"
                shift
                ;;
            *)
                log_error "未知参数: $1"
                show_help
                exit 1
                ;;
        esac
    done
    
    if [ -z "${COMMAND}" ]; then
        log_error "请指定命令"
        show_help
        exit 1
    fi
}

# 检查依赖
check_dependencies() {
    if ! command -v kubectl &> /dev/null; then
        log_error "kubectl 未安装"
        exit 1
    fi
    
    if ! command -v kind &> /dev/null; then
        log_error "kind 未安装"
        exit 1
    fi
}

# 设置 kubectl 上下文
setup_context() {
    if ! kind get clusters | grep -q "^${KIND_CLUSTER_NAME}$"; then
        log_error "Kind 集群 '${KIND_CLUSTER_NAME}' 不存在"
        exit 1
    fi
    
    kubectl config use-context "kind-${KIND_CLUSTER_NAME}"
}

# 获取 yurt-manager pod 名称
get_pod_name() {
    kubectl get pods -n "${NAMESPACE}" -l app=yurt-manager -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo ""
}

# 检查 pod 是否存在
check_pod_exists() {
    local pod_name=$(get_pod_name)
    if [ -z "${pod_name}" ]; then
        log_error "yurt-manager pod 未找到"
        return 1
    fi
    echo "${pod_name}"
}

# 获取日志
get_logs() {
    log_info "获取 yurt-manager 日志..."
    
    local pod_name=$(check_pod_exists)
    if [ $? -ne 0 ]; then
        return 1
    fi
    
    if [ "${FOLLOW_LOGS}" = "true" ]; then
        kubectl logs -n "${NAMESPACE}" "${pod_name}" -f
    else
        kubectl logs -n "${NAMESPACE}" "${pod_name}" --tail=100
    fi
}

# 检查状态
check_status() {
    log_info "检查部署状态..."
    
    echo ""
    log_info "命名空间状态:"
    kubectl get namespace "${NAMESPACE}" || log_warning "命名空间 ${NAMESPACE} 不存在"
    
    echo ""
    log_info "CRD 状态:"
    kubectl get crd tenantclusters.apps.openyurt.io || log_warning "TenantCluster CRD 不存在"
    
    echo ""
    log_info "Pod 状态:"
    kubectl get pods -n "${NAMESPACE}" -l app=yurt-manager || log_warning "yurt-manager pod 不存在"
    
    echo ""
    log_info "Service 状态:"
    kubectl get services -n "${NAMESPACE}" || log_warning "命名空间 ${NAMESPACE} 中没有服务"
    
    echo ""
    log_info "RBAC 状态:"
    kubectl get serviceaccount -n "${NAMESPACE}" yurt-manager || log_warning "ServiceAccount 不存在"
    kubectl get clusterrole yurt-manager || log_warning "ClusterRole 不存在"
    kubectl get clusterrolebinding yurt-manager || log_warning "ClusterRoleBinding 不存在"
}

# 显示事件
show_events() {
    log_info "显示相关事件..."
    
    echo ""
    log_info "命名空间事件:"
    kubectl get events -n "${NAMESPACE}" --sort-by='.lastTimestamp'
    
    echo ""
    log_info "集群事件 (最近 20 条):"
    kubectl get events --all-namespaces --sort-by='.lastTimestamp' | tail -20
}

# 描述 pod
describe_pod() {
    log_info "描述 yurt-manager pod..."
    
    local pod_name=$(check_pod_exists)
    if [ $? -ne 0 ]; then
        return 1
    fi
    
    kubectl describe pod -n "${NAMESPACE}" "${pod_name}"
}

# 检查 TenantCluster 资源
check_tenantcluster() {
    log_info "检查 TenantCluster 资源..."
    
    echo ""
    log_info "TenantCluster 列表:"
    kubectl get tenantcluster --all-namespaces || log_warning "没有找到 TenantCluster 资源"
    
    echo ""
    log_info "TenantCluster 详细信息:"
    local tc_names=$(kubectl get tenantcluster --all-namespaces -o jsonpath='{.items[*].metadata.name}' 2>/dev/null || echo "")
    if [ -n "${tc_names}" ]; then
        for tc_name in ${tc_names}; do
            local tc_namespace=$(kubectl get tenantcluster "${tc_name}" -o jsonpath='{.metadata.namespace}' 2>/dev/null || echo "default")
            echo "--- TenantCluster: ${tc_namespace}/${tc_name} ---"
            kubectl describe tenantcluster -n "${tc_namespace}" "${tc_name}"
        done
    else
        log_info "没有 TenantCluster 资源"
    fi
}

# 进入 pod shell
enter_shell() {
    log_info "进入 yurt-manager pod shell..."
    
    local pod_name=$(check_pod_exists)
    if [ $? -ne 0 ]; then
        return 1
    fi
    
    kubectl exec -n "${NAMESPACE}" -it "${pod_name}" -- /bin/bash
}

# 收集调试信息
collect_debug_info() {
    log_info "收集调试信息到 ${OUTPUT_DIR}..."
    
    # 创建输出目录
    mkdir -p "${OUTPUT_DIR}"
    
    # 生成时间戳
    local timestamp=$(date +"%Y%m%d%H%M%S")
    local debug_dir="${OUTPUT_DIR}/debug-${timestamp}"
    mkdir -p "${debug_dir}"
    
    # 收集集群信息
    log_info "收集集群信息..."
    kubectl cluster-info > "${debug_dir}/cluster-info.txt" 2>&1
    kubectl version > "${debug_dir}/version.txt" 2>&1
    kubectl get nodes -o wide > "${debug_dir}/nodes.txt" 2>&1
    
    # 收集命名空间信息
    log_info "收集命名空间信息..."
    kubectl get namespace "${NAMESPACE}" -o yaml > "${debug_dir}/namespace.yaml" 2>&1
    
    # 收集 CRD 信息
    log_info "收集 CRD 信息..."
    kubectl get crd tenantclusters.apps.openyurt.io -o yaml > "${debug_dir}/tenantcluster-crd.yaml" 2>&1
    
    # 收集 RBAC 信息
    log_info "收集 RBAC 信息..."
    kubectl get serviceaccount -n "${NAMESPACE}" yurt-manager -o yaml > "${debug_dir}/serviceaccount.yaml" 2>&1
    kubectl get clusterrole yurt-manager -o yaml > "${debug_dir}/clusterrole.yaml" 2>&1
    kubectl get clusterrolebinding yurt-manager -o yaml > "${debug_dir}/clusterrolebinding.yaml" 2>&1
    
    # 收集 pod 信息
    log_info "收集 pod 信息..."
    local pod_name=$(get_pod_name)
    if [ -n "${pod_name}" ]; then
        kubectl get pod -n "${NAMESPACE}" "${pod_name}" -o yaml > "${debug_dir}/pod.yaml" 2>&1
        kubectl describe pod -n "${NAMESPACE}" "${pod_name}" > "${debug_dir}/pod-describe.txt" 2>&1
        kubectl logs -n "${NAMESPACE}" "${pod_name}" > "${debug_dir}/pod-logs.txt" 2>&1
    fi
    
    # 收集事件
    log_info "收集事件..."
    kubectl get events -n "${NAMESPACE}" --sort-by='.lastTimestamp' > "${debug_dir}/events.txt" 2>&1
    
    # 收集 TenantCluster 资源
    log_info "收集 TenantCluster 资源..."
    kubectl get tenantcluster --all-namespaces -o yaml > "${debug_dir}/tenantclusters.yaml" 2>&1
    
    # 收集系统信息
    log_info "收集系统信息..."
    df -h > "${debug_dir}/disk-space.txt" 2>&1
    free -h > "${debug_dir}/memory.txt" 2>&1
    docker ps > "${debug_dir}/docker-containers.txt" 2>&1
    kind get nodes > "${debug_dir}/kind-nodes.txt" 2>&1
    
    # 创建摘要
    log_info "创建调试摘要..."
    cat > "${debug_dir}/README.txt" << EOF
调试信息收集时间: $(date)
Kind 集群: ${KIND_CLUSTER_NAME}
命名空间: ${NAMESPACE}

文件说明:
- cluster-info.txt: 集群信息
- version.txt: Kubernetes 版本信息
- nodes.txt: 节点信息
- namespace.yaml: 命名空间配置
- tenantcluster-crd.yaml: TenantCluster CRD 配置
- serviceaccount.yaml: ServiceAccount 配置
- clusterrole.yaml: ClusterRole 配置
- clusterrolebinding.yaml: ClusterRoleBinding 配置
- pod.yaml: Pod 配置
- pod-describe.txt: Pod 详细描述
- pod-logs.txt: Pod 日志
- events.txt: 事件信息
- tenantclusters.yaml: TenantCluster 资源
- disk-space.txt: 磁盘空间使用情况
- memory.txt: 内存使用情况
- docker-containers.txt: Docker 容器列表
- kind-nodes.txt: Kind 节点列表

常见问题排查:
1. Pod 无法启动: 检查 pod.yaml 和 pod-describe.txt
2. 权限问题: 检查 serviceaccount.yaml, clusterrole.yaml, clusterrolebinding.yaml
3. CRD 问题: 检查 tenantcluster-crd.yaml
4. 网络问题: 检查 events.txt 和 cluster-info.txt
5. 资源不足: 检查 disk-space.txt 和 memory.txt
EOF
    
    log_success "调试信息已收集到: ${debug_dir}"
    log_info "查看摘要: cat ${debug_dir}/README.txt"
}

# 清理调试日志
clean_logs() {
    log_info "清理调试日志..."
    
    if [ -d "${OUTPUT_DIR}" ]; then
        rm -rf "${OUTPUT_DIR}"
        log_success "调试日志已清理"
    else
        log_info "没有找到调试日志目录"
    fi
}

# 主函数
main() {
    # 解析参数
    parse_args "$@"
    
    # 检查依赖
    check_dependencies
    
    # 设置 kubectl 上下文
    setup_context
    
    # 执行命令
    case "${COMMAND}" in
        logs)
            get_logs
            ;;
        status)
            check_status
            ;;
        events)
            show_events
            ;;
        describe)
            describe_pod
            ;;
        tenantcluster)
            check_tenantcluster
            ;;
        collect)
            collect_debug_info
            ;;
        shell)
            enter_shell
            ;;
        clean)
            clean_logs
            ;;
        *)
            log_error "未知命令: ${COMMAND}"
            show_help
            exit 1
            ;;
    esac
}

# 执行主函数
main "$@"