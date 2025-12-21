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
CLEANUP="${CLEANUP:-false}"
SKIP_BUILD="${SKIP_BUILD:-false}"

# 显示帮助信息
show_help() {
    cat << EOF
用法: $0 [选项]

修复 TenantCluster operator 配置，只启用 TenantCluster 控制器

选项:
    -h, --help                          显示此帮助信息
    -c, --cleanup                       清理现有资源后重新部署
    -b, --skip-build                    跳过镜像构建
    -n, --namespace <namespace>         部署命名空间 (默认: openyurt-system)
    -r, --repo <repo>                   镜像仓库 (默认: openyurt)
    -t, --tag <tag>                     镜像标签 (默认: latest)
    --cluster-name <name>               Kind 集群名称 (默认: tenantcluster-test)

示例:
    $0                                  # 使用默认配置修复
    $0 -c                               # 清理并重新修复
    $0 --skip-build                     # 跳过镜像构建，直接修复
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
            -c|--cleanup)
                CLEANUP="true"
                shift
                ;;
            -b|--skip-build)
                SKIP_BUILD="true"
                shift
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

# 清理现有资源
cleanup_resources() {
    if [ "${CLEANUP}" != "true" ]; then
        return
    fi
    
    log_info "清理现有资源..."
    
    # 删除 yurt-manager deployment
    if kubectl get deployment -n "${NAMESPACE}" yurt-manager &>/dev/null; then
        log_info "删除 yurt-manager deployment"
        kubectl delete deployment -n "${NAMESPACE}" yurt-manager --ignore-not-found=true
    fi
    
    # 等待删除完成
    log_info "等待资源删除完成..."
    kubectl wait --for=delete deployment/yurt-manager -n "${NAMESPACE}" --timeout=60s || true
    
    log_success "资源清理完成"
}

# 构建 yurt-manager 镜像
build_image() {
    if [ "${SKIP_BUILD}" = "true" ]; then
        log_info "跳过镜像构建"
        return
    fi
    
    log_info "构建 yurt-manager 镜像..."
    
    cd "${OPENYURT_ROOT}"
    
    # 设置构建参数
    export TARGET_PLATFORMS="linux/amd64"
    export IMAGE_REPO="${IMAGE_REPO}"
    export IMAGE_TAG="${IMAGE_TAG}"
    
    # 构建镜像
    log_info "执行镜像构建命令..."
    if ! make docker-build WHAT=yurt-manager; then
        log_error "镜像构建失败"
        exit 1
    fi
    
    # 加载镜像到 Kind 集群
    log_info "加载镜像到 Kind 集群..."
    if ! kind load docker-image "${IMAGE_REPO}/yurt-manager:${IMAGE_TAG}" --name "${KIND_CLUSTER_NAME}"; then
        log_error "镜像加载失败"
        exit 1
    fi
    
    log_success "镜像构建并加载完成: ${IMAGE_REPO}/yurt-manager:${IMAGE_TAG}"
}

# 修复 operator 配置
fix_operator_config() {
    log_info "修复 yurt-manager operator 配置..."
    
    # 创建命名空间（如果不存在）
    if ! kubectl get namespace "${NAMESPACE}" &>/dev/null; then
        kubectl create namespace "${NAMESPACE}"
        log_success "命名空间创建成功"
    fi
    
    # 创建 ServiceAccount（如果不存在）
    if ! kubectl get serviceaccount -n "${NAMESPACE}" yurt-manager &>/dev/null; then
        kubectl apply -f - <<EOF
apiVersion: v1
kind: ServiceAccount
metadata:
  name: yurt-manager
  namespace: ${NAMESPACE}
EOF
    fi
    
    # 创建 ClusterRole（如果不存在）
    if ! kubectl get clusterrole yurt-manager &>/dev/null; then
        kubectl apply -f - <<EOF
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: yurt-manager
rules:
- apiGroups:
  - ""
  resources:
  - events
  verbs:
  - create
  - patch
- apiGroups:
  - ""
  resources:
  - secrets
  verbs:
  - get
  - list
  - watch
  - create
  - update
  - patch
  - delete
- apiGroups:
  - ""
  resources:
  - services
  verbs:
  - get
  - list
  - watch
  - create
  - update
  - patch
  - delete
- apiGroups:
  - ""
  resources:
  - namespaces
  verbs:
  - get
  - list
  - watch
  - create
  - update
  - patch
  - delete
- apiGroups:
  - ""
  resources:
  - configmaps
  verbs:
  - get
  - list
  - watch
  - create
  - update
  - patch
  - delete
- apiGroups:
  - apps
  resources:
  - deployments
  - statefulsets
  - daemonsets
  verbs:
  - get
  - list
  - watch
  - create
  - update
  - patch
  - delete
- apiGroups:
  - batch
  resources:
  - jobs
  verbs:
  - get
  - list
  - watch
  - create
  - update
  - patch
  - delete
- apiGroups:
  - apps.openyurt.io
  resources:
  - tenantclusters
  verbs:
  - get
  - list
  - watch
  - create
  - update
  - patch
  - delete
- apiGroups:
  - apps.openyurt.io
  resources:
  - tenantclusters/status
  verbs:
  - get
  - update
  - patch
- apiGroups:
  - apps.openyurt.io
  resources:
  - tenantclusters/finalizers
  verbs:
  - update
EOF
    fi
    
    # 创建 ClusterRoleBinding（如果不存在）
    if ! kubectl get clusterrolebinding yurt-manager &>/dev/null; then
        kubectl apply -f - <<EOF
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: yurt-manager
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: yurt-manager
subjects:
- kind: ServiceAccount
  name: yurt-manager
  namespace: ${NAMESPACE}
EOF
    fi
    
    # 创建或更新 yurt-manager deployment（只启用 TenantCluster 控制器）
    log_info "创建修复后的 yurt-manager deployment..."
    
    kubectl apply -f - <<EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: yurt-manager
  namespace: ${NAMESPACE}
  labels:
    app: yurt-manager
spec:
  replicas: 1
  selector:
    matchLabels:
      app: yurt-manager
  template:
    metadata:
      labels:
        app: yurt-manager
    spec:
      serviceAccountName: yurt-manager
      tolerations:
        - effect: NoSchedule
          key: node-role.kubernetes.io/master
        - effect: NoSchedule
          key: node-role.kubernetes.io/control-plane
      containers:
        - name: yurt-manager
          image: ${IMAGE_REPO}/yurt-manager:${IMAGE_TAG}
          imagePullPolicy: IfNotPresent
          command:
            - /usr/local/bin/yurt-manager
          args:
            - --metrics-addr=:10271
            - --health-probe-addr=:10272
            - --webhook-port=10273
            - --v=4
            - --working-namespace=${NAMESPACE}
            - --controllers=tenantcluster
            - --leader-elect-resource-name=cloud-yurt-manager
          ports:
            - containerPort: 10273
              name: webhook-server
              protocol: TCP
            - containerPort: 10271
              name: metrics
              protocol: TCP
            - containerPort: 10272
              name: health
              protocol: TCP
          livenessProbe:
            httpGet:
              path: /healthz
              port: 10272
            initialDelaySeconds: 60
            timeoutSeconds: 2
            periodSeconds: 10
            failureThreshold: 3
          readinessProbe:
            httpGet:
              path: /readyz
              port: 10272
            initialDelaySeconds: 60
            timeoutSeconds: 2
            periodSeconds: 10
            failureThreshold: 2
          resources:
            limits:
              cpu: 2000m
              memory: 1024Mi
            requests:
              cpu: 100m
              memory: 256Mi
EOF
    
    log_success "yurt-manager operator 配置修复完成"
}

# 验证修复
verify_fix() {
    log_info "验证修复结果..."
    
    # 检查 yurt-manager pod
    log_info "检查 yurt-manager pod..."
    local max_wait=120
    local wait_time=0
    while true; do
        local pod_status=$(kubectl get pods -n "${NAMESPACE}" -l app=yurt-manager -o jsonpath='{.items[0].status.phase}' 2>/dev/null || echo "")
        
        if [ "${pod_status}" = "Running" ]; then
            log_success "yurt-manager pod 正在运行"
            break
        elif [ "${pod_status}" = "Error" ] || [ "${pod_status}" = "CrashLoopBackOff" ]; then
            log_error "yurt-manager pod 处于错误状态: ${pod_status}"
            kubectl logs -n "${NAMESPACE}" -l app=yurt-manager --tail=50
            return 1
        fi
        
        if [ $wait_time -ge $max_wait ]; then
            log_error "yurt-manager pod 启动超时"
            kubectl get pods -n "${NAMESPACE}" -l app=yurt-manager
            return 1
        fi
        
        sleep 2
        ((wait_time++))
    done
    
    # 检查 TenantCluster controller
    log_info "检查 TenantCluster controller..."
    local controller_logs=$(kubectl logs -n "${NAMESPACE}" -l app=yurt-manager --tail=20 2>/dev/null || echo "")
    if echo "${controller_logs}" | grep -q "tenantcluster-controller"; then
        log_success "TenantCluster controller 已启动"
    else
        log_warning "TenantCluster controller 未在日志中找到，可能仍在启动中"
    fi
    
    log_success "修复验证完成"
}

# 显示修复信息
show_fix_info() {
    log_info "修复信息:"
    echo "  - 命名空间: ${NAMESPACE}"
    echo "  - 镜像: ${IMAGE_REPO}/yurt-manager:${IMAGE_TAG}"
    echo "  - Kind 集群: ${KIND_CLUSTER_NAME}"
    echo "  - 启用的控制器: tenantcluster"
    
    echo ""
    log_info "测试 TenantCluster:"
    echo "  kubectl apply -f test-env/tenantcluster-sample.yaml"
    echo "  kubectl get tenantcluster"
    echo "  kubectl describe tenantcluster <name>"
    
    echo ""
    log_info "查看日志:"
    echo "  kubectl logs -n ${NAMESPACE} -l app=yurt-manager -f"
    
    echo ""
    log_info "故障排除:"
    echo "  kubectl get pods -n ${NAMESPACE}"
    echo "  kubectl describe pod -n ${NAMESPACE} -l app=yurt-manager"
    echo "  kubectl get events -n ${NAMESPACE} --sort-by='.lastTimestamp'"
}

# 主函数
main() {
    log_info "开始修复 TenantCluster operator 配置..."
    
    # 解析参数
    parse_args "$@"
    
    # 检查依赖
    check_dependencies
    
    # 检查 Kind 集群
    check_kind_cluster
    
    # 清理资源
    cleanup_resources
    
    # 构建镜像
    build_image
    
    # 修复 operator 配置
    fix_operator_config
    
    # 验证修复
    verify_fix
    
    # 显示修复信息
    show_fix_info
    
    log_success "TenantCluster operator 配置修复完成!"
}

# 错误处理
trap 'log_error "脚本执行失败，退出码: $?"' ERR

# 执行主函数
main "$@"