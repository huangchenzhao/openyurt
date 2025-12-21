# TenantCluster 测试故障排除指南

## 概述

本指南提供了在 TenantCluster operator 测试过程中可能遇到的常见问题及其解决方案。这些问题按类别分组，包括环境设置、部署、运行时和性能问题。

## 环境设置问题

### 1. 工具安装问题

#### 问题：kubectl 未安装或版本不兼容

**症状**：
```
错误: kubectl 未安装
./install-tools.sh: line XX: kubectl: command not found
```

**解决方案**：
```bash
# 使用提供的安装脚本
./install-tools.sh -k

# 或手动安装
curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"
sudo install -o root -g root -m 0755 kubectl /usr/local/bin/kubectl

# 验证安装
kubectl version --client
```

#### 问题：kind 未安装或版本过低

**症状**：
```
错误: kind 未安装
./install-tools.sh: line XX: kind: command not found
```

**解决方案**：
```bash
# 使用提供的安装脚本
./install-tools.sh -n

# 或手动安装
curl -Lo ./kind https://kind.sigs.k8s.io/dl/v0.24.0/kind-linux-amd64
chmod +x ./kind
sudo mv ./kind /usr/local/bin/kind

# 验证安装
kind version
```

#### 问题：Docker 未运行

**症状**：
```
错误: Docker 未运行。请启动 Docker
Cannot connect to the Docker daemon
```

**解决方案**：
```bash
# 启动 Docker 服务
sudo systemctl start docker
sudo systemctl enable docker

# 验证 Docker 运行状态
docker version
docker info
```

### 2. Kind 集群问题

#### 问题：Kind 集群创建失败

**症状**：
```
ERROR: failed to create cluster: failed to create nodes: command "docker create..."
```

**解决方案**：
```bash
# 检查 Docker 资源
docker system df
docker system prune

# 检查端口冲突
netstat -tulpn | grep 31000
netstat -tulpn | grep 32000

# 修改 kind-config.yaml 中的端口映射
# 或者删除现有集群后重试
kind delete cluster --name tenantcluster-test
./setup-kind-cluster.sh
```

#### 问题：Kind 集群节点未就绪

**症状**：
```
kubectl get nodes
NAME                         STATUS     ROLES           AGE   VERSION
tenantcluster-test-control-plane   NotReady   control-plane   5m    v1.31.0
```

**解决方案**：
```bash
# 检查集群事件
kubectl get events --sort-by='.lastTimestamp'

# 检查系统 pod 状态
kubectl get pods -n kube-system

# 重启集群
kind delete cluster --name tenantcluster-test
./setup-kind-cluster.sh
```

## 部署问题

### 1. CRD 部署问题

#### 问题：TenantCluster CRD 未安装

**症状**：
```
error: the server doesn't have a resource type "tenantclusters"
```

**解决方案**：
```bash
# 手动部署 CRD
kubectl apply -f ${OPENYURT_ROOT}/charts/yurt-manager/crds/apiextensions.k8s.io_v1_customresourcedefinition_tenantclusters.apps.openyurt.io.yaml

# 验证 CRD 安装
kubectl get crd tenantclusters.apps.openyurt.io
```

#### 问题：CRD 安装冲突

**症状**：
```
error: error validating "CRD.yaml": error validating data: ValidationError(CustomResourceDefinition.spec.versions[0].name): unknown field "served"
```

**解决方案**：
```bash
# 删除现有 CRD
kubectl delete crd tenantclusters.apps.openyurt.io

# 重新安装
kubectl apply -f ${OPENYURT_ROOT}/charts/yurt-manager/crds/apiextensions.k8s.io_v1_customresourcedefinition_tenantclusters.apps.openyurt.io.yaml
```

### 2. Operator 部署问题

#### 问题：控制器依赖问题（最常见）

**症状**：
```
yurt-manager pod 处于 CrashLoopBackOff 状态
日志显示: resource apps.openyurt.io/v1alpha1, Resource=yurtstaticsets doesn't exist
```

**解决方案**：
```bash
# 使用修复脚本
./fix-operator-config.sh --cleanup

# 或手动修复
kubectl patch deployment yurt-manager -n openyurt-system --type='json' -p='[{"op": "replace", "path": "/spec/template/spec/containers/0/args", "value": ["--metrics-addr=:10271", "--health-probe-addr=:10272", "--webhook-port=10273", "--v=4", "--working-namespace=openyurt-system", "--controllers=tenantcluster", "--leader-elect-resource-name=cloud-yurt-manager"]}]'
```

#### 问题：镜像拉取失败

**症状**：
```
Failed to pull image "openyurt/yurt-manager:latest": rpc error: code = Unknown
```

**解决方案**：
```bash
# 构建本地镜像
cd ${OPENYURT_ROOT}
make docker-build WHAT=yurt-manager

# 加载到 Kind 集群
kind load docker-image openyurt/yurt-manager:latest --name tenantcluster-test

# 或使用修复脚本（包含构建步骤）
./fix-operator-config.sh --cleanup
```

#### 问题：RBAC 权限问题

**症状**：
```
error validating data: ValidationError(Deployment.spec.template.spec.serviceAccountName): unknown field "serviceAccountName"
```

**解决方案**：
```bash
# 检查 ServiceAccount
kubectl get serviceaccount -n openyurt-system

# 检查 ClusterRole
kubectl get clusterrole yurt-manager

# 检查 ClusterRoleBinding
kubectl get clusterrolebinding yurt-manager

# 重新创建 RBAC 资源
./fix-operator-config.sh --cleanup
```

## 运行时问题

### 1. TenantCluster 创建问题

#### 问题：CA Secret 不存在

**症状**：
```
TenantCluster 创建失败，事件显示: secret "host-k8s-ca" not found
```

**解决方案**：
```bash
# 创建 CA secret
./create-ca-secret.sh

# 验证 secret
kubectl get secret host-k8s-ca -n kube-system
```

#### 问题：存储类不存在

**症状**：
```
TenantCluster 创建失败，事件显示: storageclass "local-path" not found
```

**解决方案**：
```bash
# 安装 local-path-provisioner
./install-local-path-provisioner.sh

# 验证存储类
kubectl get storageclass
```

#### 问题：资源配额不足

**症状**：
```
TenantCluster 创建失败，事件显示: exceeded quota
```

**解决方案**：
```bash
# 检查资源配额
kubectl describe quota

# 检查资源使用
kubectl top nodes
kubectl top pods -A

# 清理不必要的资源
kubectl delete tenantcluster --all
```

### 2. 控制平面组件问题

#### 问题：API Server 无法启动

**症状**：
```
tenant-apiserver pod 处于 CrashLoopBackOff 状态
日志显示: failed to create listener: failed to listen on 0.0.0.0:443
```

**解决方案**：
```bash
# 检查端口占用
netstat -tulpn | grep 443

# 检查服务配置
kubectl get service -n <tenant-namespace>
kubectl describe service tenant-apiserver -n <tenant-namespace>

# 检查证书
kubectl get secret -n <tenant-namespace> | grep apiserver
```

#### 问题：ETCD 无法启动

**症状**：
```
tenant-etcd pod 处于 CrashLoopBackOff 状态
日志显示: mvcc: database space exceeded
```

**解决方案**：
```bash
# 检查 PVC 状态
kubectl get pvc -n <tenant-namespace>

# 扩展存储空间
kubectl patch tenantcluster <name> -n <tenant-namespace> --type='merge' -p='{"spec":{"etcdStorageSize":"2Gi"}}'

# 或者删除并重建
kubectl delete tenantcluster <name> -n <tenant-namespace>
```

#### 问题：Controller Manager 无法启动

**症状**：
```
tenant-controller-manager pod 处于 CrashLoopBackOff 状态
日志显示: failed to get serviceaccount
```

**解决方案**：
```bash
# 检查 RBAC 配置
kubectl get serviceaccount -n <tenant-namespace>
kubectl get role -n <tenant-namespace>
kubectl get rolebinding -n <tenant-namespace>

# 重新创建 TenantCluster
kubectl delete tenantcluster <name> -n <tenant-namespace>
```

### 3. 网络问题

#### 问题：API Server 无法访问

**症状**：
```
使用 kubeconfig 连接失败: connection refused
```

**解决方案**：
```bash
# 检查服务状态
kubectl get service tenant-apiserver -n <tenant-namespace>
kubectl describe service tenant-apiserver -n <tenant-namespace>

# 检查端点
kubectl get endpoints tenant-apiserver -n <tenant-namespace>

# 检查网络策略
kubectl get networkpolicy -n <tenant-namespace>
```

#### 问题：Pod 无法访问外部网络

**症状**：
```
Pod 中无法访问外部服务: connection timeout
```

**解决方案**：
```bash
# 检查 DNS 配置
kubectl get configmap -n kube-system coredns -o yaml

# 检查网络插件
kubectl get pods -n kube-system | grep -E "(calico|flannel|weave)"

# 测试网络连通性
kubectl run test-pod --image=busybox --rm -it -- /bin/sh
# 在 pod 中执行:
# nslookup kubernetes.default.svc.cluster.local
# wget -qO- http://example.com
```

## 性能问题

### 1. 启动时间过长

#### 问题：Operator 启动时间过长

**症状**：
```
yurt-manager pod 启动时间超过 5 分钟
```

**解决方案**：
```bash
# 检查资源限制
kubectl describe pod -n openyurt-system -l app=yurt-manager

# 增加资源限制
kubectl patch deployment yurt-manager -n openyurt-system --type='merge' -p='{"spec":{"template":{"spec":{"containers":[{"name":"yurt-manager","resources":{"requests":{"cpu":"200m","memory":"512Mi"}}}]}}}'

# 检查镜像拉取时间
kubectl get events -n openyurt-system --sort-by='.lastTimestamp'
```

#### 问题：TenantCluster 创建时间过长

**症状**：
```
TenantCluster 从创建到 Running 状态超过 10 分钟
```

**解决方案**：
```bash
# 检查资源使用
kubectl top nodes
kubectl top pods -A

# 检查存储性能
kubectl get pvc -A
kubectl describe pvc <pvc-name> -n <namespace>

# 增加副本数
kubectl patch tenantcluster <name> -n <namespace> --type='merge' -p='{"spec":{"etcdReplicas":1}}'
```

### 2. 资源使用过高

#### 问题：内存使用过高

**症状**：
```
Pod 内存使用持续增长，最终被 OOMKilled
```

**解决方案**：
```bash
# 检查内存使用
kubectl top pods -A --sort-by=memory

# 增加内存限制
kubectl patch deployment <deployment-name> -n <namespace> --type='merge' -p='{"spec":{"template":{"spec":{"containers":[{"name":"<container-name>","resources":{"limits":{"memory":"1Gi"}}}]}}}'

# 检查内存泄漏
kubectl logs -n <namespace> <pod-name> --previous
```

#### 问题：CPU 使用过高

**症状**：
```
Pod CPU 使用持续高于 80%
```

**解决方案**：
```bash
# 检查 CPU 使用
kubectl top pods -A --sort-by=cpu

# 增加 CPU 限制
kubectl patch deployment <deployment-name> -n <namespace> --type='merge' -p='{"spec":{"template":{"spec":{"containers":[{"name":"<container-name>","resources":{"limits":{"cpu":"1000m"}}}]}}}'

# 检查是否有无限循环
kubectl logs -n <namespace> <pod-name> | tail -100
```

## 调试技巧

### 1. 日志收集

#### 收集所有相关日志
```bash
# 创建日志目录
mkdir -p debug-logs

# 收集系统 pod 日志
kubectl get pods -n kube-system -o name | xargs -I {} sh -c 'kubectl logs -n kube-system {} --tail=100 > debug-logs/{}.log'

# 收集 operator 日志
kubectl logs -n openyurt-system -l app=yurt-manager --tail=1000 > debug-logs/yurt-manager.log

# 收集 tenant pod 日志
kubectl get pods -A -l tenant= -o name | xargs -I {} sh -c 'kubectl logs -n $(echo {} | cut -d/ -f1) $(echo {} | cut -d/ -f2) --tail=100 > debug-logs/{}.log'
```

#### 启用详细日志
```bash
# 修改 operator 日志级别
kubectl patch deployment yurt-manager -n openyurt-system --type='json' -p='[{"op": "replace", "path": "/spec/template/spec/containers/0/args", "value": ["--metrics-addr=:10271", "--health-probe-addr=:10272", "--webhook-port=10273", "--v=4", "--working-namespace=openyurt-system", "--controllers=tenantcluster", "--leader-elect-resource-name=cloud-yurt-manager"]}]'

# 修改 tenant 组件日志级别
kubectl patch tenantcluster <name> -n <namespace> --type='merge' -p='{"spec":{"logLevel":4}}'
```

### 2. 状态检查

#### 检查资源状态
```bash
# 检查所有相关资源
kubectl get all,secrets,configmaps,pvc -A | grep -E "(tenant|yurt)"

# 检查事件
kubectl get events -A --sort-by='.lastTimestamp'

# 检查资源描述
kubectl describe tenantcluster <name> -n <namespace>
kubectl describe pod -n <namespace> <pod-name>
kubectl describe service -n <namespace> <service-name>
```

#### 检查网络状态
```bash
# 检查服务端点
kubectl get endpoints -A

# 检查网络策略
kubectl get networkpolicy -A

# 测试内部连通性
kubectl run test-pod --image=busybox --rm -it -- /bin/sh
# 在 pod 中执行:
# nslookup kubernetes.default.svc.cluster.local
# wget -qO- http://<service-name>.<namespace>.svc.cluster.local
```

### 3. 性能分析

#### 资源使用分析
```bash
# 实时监控资源使用
watch -n 5 'kubectl top pods -A --sort-by=cpu'
watch -n 5 'kubectl top pods -A --sort-by=memory'

# 节点资源使用
kubectl describe nodes

# 存储使用情况
kubectl df
```

#### 网络性能分析
```bash
# 网络延迟测试
kubectl exec -it <pod-name> -n <namespace> -- ping <target-ip>

# 带宽测试
kubectl exec -it <pod-name> -n <namespace> -- wget -O /dev/null http://speedtest.net
```

## 恢复程序

### 1. 部分恢复

#### 恢复单个 TenantCluster
```bash
# 删除失败的 TenantCluster
kubectl delete tenantcluster <name> -n <namespace> --wait=false

# 清理相关资源
kubectl delete namespace <namespace> --wait=false

# 重新创建
kubectl apply -f <tenantcluster-yaml>
```

#### 恢复控制平面组件
```bash
# 重启特定组件
kubectl rollout restart deployment/<component-name> -n <namespace>

# 检查重启状态
kubectl rollout status deployment/<component-name> -n <namespace>
```

### 2. 完全恢复

#### 完全重置测试环境
```bash
# 删除所有 TenantCluster
kubectl get tenantcluster -A -o name | xargs -I {} kubectl delete {} --wait=false

# 删除相关命名空间
kubectl get namespaces -o name | grep tc-test | xargs -I {} kubectl delete {} --wait=false

# 重置 operator
./fix-operator-config.sh --cleanup

# 重置 Kind 集群（如果需要）
kind delete cluster --name tenantcluster-test
./setup-kind-cluster.sh
```

## 联系支持

如果问题仍然存在，请收集以下信息并提交 issue：

1. **环境信息**：
   ```bash
   uname -a
   docker version
   kind version
   kubectl version
   ```

2. **错误日志**：
   ```bash
   # 收集所有日志
   ./debug-operator.sh collect > debug-info.txt
   ```

3. **资源状态**：
   ```bash
   # 收集资源状态
   kubectl get all,pvc,secrets,events -A > resources.txt
   ```

4. **配置文件**：
   - kind-config.yaml
   - tenantcluster-sample.yaml
   - 任何自定义配置文件

---

**文档更新时间**: 2025-12-21T10:05:00Z
**版本**: TenantCluster v1alpha1