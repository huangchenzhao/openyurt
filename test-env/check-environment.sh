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
REPORT_FILE="./test-env/environment-report.txt"

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
}

# Check prerequisites
check_prerequisites() {
    info "检查先决条件..."
    
    # Check if kubectl is installed
    if ! command -v kubectl &> /dev/null; then
        error "kubectl 未安装"
        return 1
    fi
    
    # Check if kind is installed
    if ! command -v kind &> /dev/null; then
        error "kind 未安装"
        return 1
    fi
    
    # Check if docker is running
    if ! docker info &> /dev/null; then
        error "Docker 未运行"
        return 1
    fi
    
    success "所有先决条件检查通过"
    return 0
}

# Check cluster status
check_cluster_status() {
    info "检查集群状态..."
    
    # Check if cluster exists
    if ! kubectl config get-contexts | grep -q "kind-$CLUSTER_NAME"; then
        error "集群 kind-$CLUSTER_NAME 不存在"
        return 1
    fi
    
    # Set kubectl context to the kind cluster
    kubectl config use-context kind-"$CLUSTER_NAME"
    
    # Check cluster info
    local cluster_info=$(kubectl cluster-info 2>&1)
    if echo "$cluster_info" | grep -q "is running"; then
        success "集群正在运行"
    else
        error "集群未正常运行"
        return 1
    fi
    
    # Check nodes
    local node_count=$(kubectl get nodes --no-headers | wc -l)
    local ready_nodes=$(kubectl get nodes --no-headers | grep " Ready" | wc -l)
    
    if [ "$node_count" -eq "$ready_nodes" ] && [ "$node_count" -gt 0 ]; then
        success "所有节点 ($ready_nodes/$node_count) 已就绪"
    else
        warning "部分节点未就绪 ($ready_nodes/$node_count)"
    fi
    
    return 0
}

# Check cluster resources
check_cluster_resources() {
    info "检查集群资源..."
    
    # Get node resource information
    kubectl top nodes 2>/dev/null || warning "无法获取节点资源使用情况 (metrics-server 可能未安装)"
    
    # Show node details
    kubectl get nodes -o wide
    
    # Show system pods
    kubectl get pods -n kube-system
    
    return 0
}

# Check storage configuration
check_storage_configuration() {
    info "检查存储配置..."
    
    # Check storage classes
    local storage_classes=$(kubectl get storageclass --no-headers 2>/dev/null | wc -l)
    if [ "$storage_classes" -gt 0 ]; then
        success "找到 $storage_classes 个存储类"
        kubectl get storageclass
    else
        warning "未找到存储类"
    fi
    
    # Check local-path-provisioner
    if kubectl get storageclass local-path &> /dev/null; then
        success "local-path 存储类已配置"
    else
        warning "local-path 存储类未配置"
    fi
    
    # Check PVs and PVCs
    local pv_count=$(kubectl get pv --no-headers 2>/dev/null | wc -l)
    local pvc_count=$(kubectl get pvc --all-namespaces --no-headers 2>/dev/null | wc -l)
    
    info "当前有 $pv_count 个持久卷和 $pvc_count 个持久卷声明"
    
    return 0
}

# Check network configuration
check_network_configuration() {
    info "检查网络配置..."
    
    # Check cluster network ranges
    local pod_cidr=$(kubectl cluster-info dump | grep -m1 "cluster-cidr" | sed 's/.*--cluster-cidr=\([^[:space:]]*\).*/\1/' || echo "未知")
    local service_cidr=$(kubectl cluster-info dump | grep -m1 "service-cluster-ip-range" | sed 's/.*--service-cluster-ip-range=\([^[:space:]]*\).*/\1/' || echo "未知")
    
    info "Pod CIDR: $pod_cidr"
    info "Service CIDR: $service_cidr"
    
    # Check CNI
    local cni_pods=$(kubectl get pods -n kube-system -l k8s-app=canal --no-headers 2>/dev/null | wc -l)
    if [ "$cni_pods" -gt 0 ]; then
        success "Canal CNI 已安装 ($cni_pods 个 pod)"
    else
        cni_pods=$(kubectl get pods -n kube-system -l k8s-app=flannel --no-headers 2>/dev/null | wc -l)
        if [ "$cni_pods" -gt 0 ]; then
            success "Flannel CNI 已安装 ($cni_pods 个 pod)"
        else
            warning "未检测到标准 CNI (Canal/Flannel)"
        fi
    fi
    
    # Check DNS
    local dns_pods=$(kubectl get pods -n kube-system -l k8s-app=kube-dns --no-headers 2>/dev/null | wc -l)
    if [ "$dns_pods" -gt 0 ]; then
        success "CoreDNS 已安装 ($dns_pods 个 pod)"
    else
        warning "CoreDNS 未检测到"
    fi
    
    return 0
}

# Check CA secret
check_ca_secret() {
    info "检查 CA secret..."
    
    # Check if host-k8s-ca secret exists
    if kubectl get secret host-k8s-ca -n kube-system &> /dev/null; then
        success "host-k8s-ca secret 已存在于 kube-system 命名空间"
        
        # Check if it contains the required keys
        local ca_crt=$(kubectl get secret host-k8s-ca -n kube-system -o jsonpath='{.data.ca\.crt}' 2>/dev/null)
        local ca_key=$(kubectl get secret host-k8s-ca -n kube-system -o jsonpath='{.data.ca\.key}' 2>/dev/null)
        
        if [ -n "$ca_crt" ] && [ -n "$ca_key" ]; then
            success "CA secret 包含所需的 ca.crt 和 ca.key"
        else
            warning "CA secret 缺少必要的证书文件"
        fi
    else
        warning "host-k8s-ca secret 不存在"
    fi
    
    return 0
}

# Check TenantCluster prerequisites
check_tenantcluster_prerequisites() {
    info "检查 TenantCluster 前提条件..."
    
    # Check if TenantCluster CRD is installed
    if kubectl get crd tenantclusters.apps.openyurt.io &> /dev/null; then
        success "TenantCluster CRD 已安装"
    else
        warning "TenantCluster CRD 未安装"
    fi
    
    # Check available NodePorts
    local used_nodeports=$(kubectl get svc --all-namespaces -o jsonpath='{.items[*].spec.ports[*].nodePort}' 2>/dev/null | tr ' ' '\n' | sort -n | uniq || echo "")
    local required_nodeport="31000"
    
    if echo "$used_nodeports" | grep -q "^$required_nodeport$"; then
        warning "NodePort $required_nodeport 已被使用"
    else
        success "NodePort $required_nodeport 可用"
    fi
    
    # Check available Service IP
    local service_ip="10.96.100.1"
    local used_service_ips=$(kubectl get svc --all-namespaces -o jsonpath='{.items[*].spec.clusterIP}' 2>/dev/null | tr ' ' '\n' | sort -n | uniq || echo "")
    
    if echo "$used_service_ips" | grep -q "^$service_ip$"; then
        warning "Service IP $service_ip 已被使用"
    else
        success "Service IP $service_ip 可用"
    fi
    
    return 0
}

# Generate report
generate_report() {
    info "生成环境检查报告..."
    
    # Create report header
    cat > "$REPORT_FILE" <<EOF
========================================
TenantCluster 测试环境检查报告
========================================
生成时间: $(date)
集群名称: $CLUSTER_NAME
========================================

1. 集群版本和状态
----------------------------------------
EOF

    # Add cluster version info
    kubectl version --output=json >> "$REPORT_FILE" 2>&1
    echo "" >> "$REPORT_FILE"
    
    # Add cluster status
    kubectl cluster-info >> "$REPORT_FILE" 2>&1
    echo "" >> "$REPORT_FILE"
    
    # Add node information
    echo "节点信息:" >> "$REPORT_FILE"
    kubectl get nodes -o wide >> "$REPORT_FILE" 2>&1
    echo "" >> "$REPORT_FILE"
    
    # Add resources section
    cat >> "$REPORT_FILE" <<EOF
2. 可用资源
----------------------------------------
EOF
    
    # Add resource usage if available
    if kubectl top nodes &> /dev/null; then
        echo "节点资源使用情况:" >> "$REPORT_FILE"
        kubectl top nodes >> "$REPORT_FILE" 2>&1
    else
        echo "无法获取资源使用情况 (metrics-server 可能未安装)" >> "$REPORT_FILE"
    fi
    echo "" >> "$REPORT_FILE"
    
    # Add storage section
    cat >> "$REPORT_FILE" <<EOF
3. 存储类配置
----------------------------------------
EOF
    
    kubectl get storageclass >> "$REPORT_FILE" 2>&1
    echo "" >> "$REPORT_FILE"
    
    # Add network section
    cat >> "$REPORT_FILE" <<EOF
4. 网络配置
----------------------------------------
EOF
    
    # Add network info
    local pod_cidr=$(kubectl cluster-info dump | grep -m1 "cluster-cidr" | sed 's/.*--cluster-cidr=\([^[:space:]]*\).*/\1/' || echo "未知")
    local service_cidr=$(kubectl cluster-info dump | grep -m1 "service-cluster-ip-range" | sed 's/.*--service-cluster-ip-range=\([^[:space:]]*\).*/\1/' || echo "未知")
    
    echo "Pod CIDR: $pod_cidr" >> "$REPORT_FILE"
    echo "Service CIDR: $service_cidr" >> "$REPORT_FILE"
    echo "" >> "$REPORT_FILE"
    
    # Add system pods
    echo "系统 Pod 状态:" >> "$REPORT_FILE"
    kubectl get pods -n kube-system >> "$REPORT_FILE" 2>&1
    echo "" >> "$REPORT_FILE"
    
    # Add secrets section
    cat >> "$REPORT_FILE" <<EOF
5. CA Secret 配置
----------------------------------------
EOF
    
    if kubectl get secret host-k8s-ca -n kube-system &> /dev/null; then
        echo "host-k8s-ca secret 存在" >> "$REPORT_FILE"
        kubectl get secret host-k8s-ca -n kube-system -o yaml >> "$REPORT_FILE" 2>&1
    else
        echo "host-k8s-ca secret 不存在" >> "$REPORT_FILE"
    fi
    echo "" >> "$REPORT_FILE"
    
    # Add TenantCluster section
    cat >> "$REPORT_FILE" <<EOF
6. TenantCluster 前提条件
----------------------------------------
EOF
    
    if kubectl get crd tenantclusters.apps.openyurt.io &> /dev/null; then
        echo "TenantCluster CRD 已安装" >> "$REPORT_FILE"
        kubectl get crd tenantclusters.apps.openyurt.io -o yaml >> "$REPORT_FILE" 2>&1
    else
        echo "TenantCluster CRD 未安装" >> "$REPORT_FILE"
    fi
    echo "" >> "$REPORT_FILE"
    
    success "环境检查报告已生成: $REPORT_FILE"
}

# Main function
main() {
    info "开始环境检查..."
    
    local all_checks_passed=true
    
    check_prerequisites || all_checks_passed=false
    check_cluster_status || all_checks_passed=false
    check_cluster_resources || all_checks_passed=false
    check_storage_configuration || all_checks_passed=false
    check_network_configuration || all_checks_passed=false
    check_ca_secret || all_checks_passed=false
    check_tenantcluster_prerequisites || all_checks_passed=false
    
    generate_report
    
    if [ "$all_checks_passed" = true ]; then
        success "所有环境检查完成！"
    else
        warning "环境检查完成，但发现一些问题，请查看报告了解详情"
    fi
    
    info "详细报告已保存到: $REPORT_FILE"
}

# Run main function
main "$@"