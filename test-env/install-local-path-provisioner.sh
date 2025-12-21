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
LOCAL_PATH_VERSION="v0.0.26"
DATA_DIR="./test-env/data"

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
    
    # Create data directory if it doesn't exist
    mkdir -p "$DATA_DIR"
    
    success "所有先决条件检查通过"
}

# Download local-path-provisioner manifest
download_manifest() {
    info "下载 local-path-provisioner manifest..."
    
    local manifest_url="https://raw.githubusercontent.com/rancher/local-path-provisioner/${LOCAL_PATH_VERSION}/deploy/local-path-storage.yaml"
    local manifest_file="./test-env/local-path-storage.yaml"
    
    # Download the manifest
    if command -v curl &> /dev/null; then
        curl -fsSL "$manifest_url" -o "$manifest_file"
    elif command -v wget &> /dev/null; then
        wget -O "$manifest_file" "$manifest_url"
    else
        error "需要 curl 或 wget 来下载 manifest"
    fi
    
    success "Manifest 下载完成: $manifest_file"
}

# Customize the manifest for kind cluster
customize_manifest() {
    info "自定义 local-path-provisioner 配置..."
    
    local manifest_file="./test-env/local-path-storage.yaml"
    
    # Create a customized manifest
    cat > "$manifest_file" <<EOF
apiVersion: v1
kind: Namespace
metadata:
  name: local-path-storage
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: local-path-provisioner-service-account
  namespace: local-path-storage
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: local-path-provisioner-role
rules:
- apiGroups: [""]
  resources: ["nodes", "persistentvolumeclaims", "configmaps"]
  verbs: ["get", "list", "watch"]
- apiGroups: [""]
  resources: ["endpoints", "persistentvolumes", "pods"]
  verbs: ["*"]
- apiGroups: [""]
  resources: ["events"]
  verbs: ["create", "patch"]
- apiGroups: ["storage.k8s.io"]
  resources: ["storageclasses"]
  verbs: ["get", "list", "watch"]
- apiGroups: ["apps"]
  resources: ["daemonsets"]
  verbs: ["get", "list", "watch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: local-path-provisioner-bind
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: local-path-provisioner-role
subjects:
- kind: ServiceAccount
  name: local-path-provisioner-service-account
  namespace: local-path-storage
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: local-path-provisioner
  namespace: local-path-storage
spec:
  replicas: 1
  selector:
    matchLabels:
      app: local-path-provisioner
  template:
    metadata:
      labels:
        app: local-path-provisioner
    spec:
      serviceAccountName: local-path-provisioner-service-account
      containers:
      - name: local-path-provisioner
        image: rancher/local-path-provisioner:${LOCAL_PATH_VERSION}
        imagePullPolicy: IfNotPresent
        command:
        - local-path-provisioner
        - --debug
        - start
        - --config
        - /etc/config/config.json
        volumeMounts:
        - name: config-volume
          mountPath: /etc/config/
        env:
        - name: POD_NAMESPACE
          valueFrom:
            fieldRef:
              fieldPath: metadata.namespace
      volumes:
      - name: config-volume
        configMap:
          name: local-path-config
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: local-path-config
  namespace: local-path-storage
data:
  config.json: |-
    {
            "nodePathMap":[
            {
                    "node":"DEFAULT_PATH_FOR_NON_LISTED_NODES",
                    "paths":["/var/local-path-provisioner"]
            }
            ]
    }
  setup: |-
    #!/bin/sh
    while getopts "m:s:p:" opt
    do
        case \$opt in
            s)
               size=\$OPTARG
            ;;
            m)
               mode=\$OPTARG
            ;;
            p)
               path=\$OPTARG
            ;;
        esac
    done

    mkdir -m \${mode:-0777} -p \${path}
    chmod 0777 \${path}
    chown 1001:1001 \${path}
  teardown: |-
    #!/bin/sh
    while getopts "m:s:p:" opt
    do
        case \$opt in
            s)
               size=\$OPTARG
            ;;
            m)
               mode=\$OPTARG
            ;;
            p)
               path=\$OPTARG
            ;;
        esac
    done

    rm -rf \${path}
  helperPod.yaml: |-
    apiVersion: v1
    kind: Pod
    metadata:
      name: helper-pod
    spec:
      containers:
      - name: helper-pod
        image: busybox
        command:
        - tail
        - -f
        - /dev/null
        imagePullPolicy: IfNotPresent
---
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: local-path
provisioner: rancher.io/local-path
volumeBindingMode: WaitForFirstConsumer
reclaimPolicy: Delete
---
kind: DaemonSet
apiVersion: apps/v1
metadata:
  name: local-path-provisioner
  namespace: local-path-storage
spec:
  selector:
    matchLabels:
      app: local-path-provisioner
  template:
    metadata:
      labels:
        app: local-path-provisioner
    spec:
      serviceAccountName: local-path-provisioner-service-account
      containers:
      - name: local-path-provisioner
        image: rancher/local-path-provisioner:${LOCAL_PATH_VERSION}
        imagePullPolicy: IfNotPresent
        command:
        - local-path-provisioner
        - --debug
        - start
        - --config
        - /etc/config/config.json
        volumeMounts:
        - name: config-volume
          mountPath: /etc/config/
        - name: data-path
          mountPath: /var/local-path-provisioner
        env:
        - name: POD_NAMESPACE
          valueFrom:
            fieldRef:
              fieldPath: metadata.namespace
      volumes:
      - name: config-volume
        configMap:
          name: local-path-config
      - name: data-path
        hostPath:
          path: /var/local-path-provisioner
          type: DirectoryOrCreate
EOF

    success "Manifest 自定义完成"
}

# Install local-path-provisioner
install_local_path_provisioner() {
    info "安装 local-path-provisioner..."
    
    # Set kubectl context to the kind cluster
    kubectl config use-context kind-"$CLUSTER_NAME"
    
    # Apply the manifest
    kubectl apply -f "./test-env/local-path-storage.yaml"
    
    success "local-path-provisioner 安装完成"
}

# Verify installation
verify_installation() {
    info "验证 local-path-provisioner 安装..."
    
    # Wait for the pod to be ready
    kubectl wait --for=condition=ready pod -l app=local-path-provisioner -n local-path-storage --timeout=300s
    
    # Check the storage class
    kubectl get storageclass local-path -o yaml
    
    # Check the provisioner pod
    kubectl get pods -n local-path-storage -o wide
    
    success "local-path-provisioner 验证完成"
}

# Test storage
test_storage() {
    info "测试存储功能..."
    
    # Create a test PVC
    cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: test-pvc
  namespace: default
spec:
  accessModes:
    - ReadWriteOnce
  storageClassName: local-path
  resources:
    requests:
      storage: 100Mi
EOF

    # Wait for PVC to be bound
    kubectl wait --for=condition=Bound pvc/test-pvc --timeout=120s
    
    # Show PVC status
    kubectl get pvc test-pvc -o wide
    
    # Clean up test PVC
    kubectl delete pvc test-pvc
    
    success "存储测试完成"
}

# Show storage information
show_storage_info() {
    info "存储信息:"
    info "  存储类名称: local-path"
    info "  默认回收策略: Delete"
    info "  卷绑定模式: WaitForFirstConsumer"
    info "  数据目录: /var/local-path-provisioner"
    
    info "在 TenantCluster CR 中使用此存储类的示例:"
    cat <<EOF
apiVersion: apps.openyurt.io/v1alpha1
kind: TenantCluster
metadata:
  name: my-tenant
spec:
  etcdStorageClass: "local-path"
  etcdStorageSize: "1Gi"
  # ... 其他配置
EOF
}

# Main function
main() {
    info "开始安装 local-path-provisioner..."
    
    check_prerequisites
    download_manifest
    customize_manifest
    install_local_path_provisioner
    verify_installation
    test_storage
    show_storage_info
    
    success "local-path-provisioner 安装和测试完成！"
}

# Run main function
main "$@"