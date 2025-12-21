# TenantCluster 测试环境

这个目录包含了用于准备 TenantCluster operator 测试环境的脚本和配置文件。

## 概述

这些脚本将自动设置一个完整的 Kubernetes 测试环境，包括：

1. 使用 kind 创建本地 Kubernetes 集群
2. 提取主机集群的 CA 证书和密钥
3. 创建 host-k8s-ca secret
4. 安装 local-path-provisioner 存储
5. 检查环境配置
6. 生成详细的环境报告

## 前提条件

在运行这些脚本之前，请确保已安装以下工具：

- Docker (正在运行)
- kind (Kubernetes in Docker)
- kubectl
- curl 或 wget (用于下载文件)

### 自动安装工具

如果您的系统上尚未安装这些工具，可以使用提供的安装脚本：

```bash
cd test-env
./install-tools.sh
```

此脚本将自动安装：
- kubectl v1.32.0
- kind v0.24.0
- 检查 Docker 是否正在运行

您也可以选择性地安装特定工具：

```bash
# 仅安装 kubectl
./install-tools.sh -k

# 仅安装 kind
./install-tools.sh -n

# 跳过 Docker 检查
./install-tools.sh -s
```

## 快速开始

### 一键设置整个环境

```bash
cd test-env
./setup-test-env.sh
```

这将执行所有步骤来设置完整的测试环境。

### 分步执行

如果您想分步执行，可以使用 `-s` 选项：

```bash
# 仅执行步骤 1 (创建 kind 集群)
./setup-test-env.sh -s 1

# 执行步骤 1, 2, 3
./setup-test-env.sh -s 1 -s 2 -s 3
```

### 仅检查环境

如果您只想检查现有环境而不进行设置：

```bash
./setup-test-env.sh -c
```

## 详细步骤说明

### 步骤 1: 创建 kind 集群

```bash
./setup-kind-cluster.sh
```

此脚本将：
- 检查先决条件（kind, kubectl, docker）
- 使用 [`kind-config.yaml`](kind-config.yaml) 创建 kind 集群
- 配置 kubectl 上下文
- 验证集群状态

### 步骤 2: 提取 CA 证书

```bash
./extract-ca-certs.sh
```

此脚本将：
- 从 kind 集群控制平面容器中提取 CA 证书和密钥
- 将证书保存到 `./certs/` 目录
- 验证证书的有效性

### 步骤 3: 创建 CA secret

```bash
./create-ca-secret.sh
```

此脚本将：
- 使用提取的 CA 证书和密钥创建 `host-k8s-ca` secret
- 将 secret 创建在 `kube-system` 命名空间中
- 验证 secret 的内容

### 步骤 4: 安装 local-path-provisioner

```bash
./install-local-path-provisioner.sh
```

此脚本将：
- 下载并自定义 local-path-provisioner 配置
- 安装 local-path-provisioner
- 创建 `local-path` 存储类
- 测试存储功能

### 步骤 5: 检查环境

```bash
./check-environment.sh
```

此脚本将：
- 检查集群状态和资源
- 验证存储和网络配置
- 检查 CA secret
- 验证 TenantCluster 前提条件

### 步骤 6: 生成报告

环境检查脚本会自动生成详细报告，保存在 `./environment-report.txt`。

## 配置文件

### kind-config.yaml

Kind 集群配置文件，定义了：
- 集群名称: `tenantcluster-test`
- 网络配置: Pod CIDR (10.244.0.0/16), Service CIDR (10.96.0.0/12)
- 端口映射: 31000, 32000
- 数据卷挂载

## 文件结构

```
test-env/
├── README.md                           # 本文件
├── kind-config.yaml                    # Kind 集群配置
├── setup-test-env.sh                   # 主脚本，整合所有步骤
├── install-tools.sh                    # 安装必要工具
├── setup-kind-cluster.sh               # 步骤 1: 创建 kind 集群
├── extract-ca-certs.sh                 # 步骤 2: 提取 CA 证书
├── create-ca-secret.sh                 # 步骤 3: 创建 CA secret
├── install-local-path-provisioner.sh   # 步骤 4: 安装存储
├── check-environment.sh                # 步骤 5: 检查环境
├── build-and-deploy-operator.sh        # 构建和部署 TenantCluster operator
├── debug-operator.sh                   # 调试和故障排除工具
├── test-deployment.sh                  # 测试部署脚本功能
├── test-tenantcluster.sh              # TenantCluster 功能测试脚本
├── run-tenantcluster-tests.sh         # 测试运行器（简化版）
├── README-test-tenantcluster.md        # 测试脚本详细说明
├── tenantcluster-sample.yaml           # 示例 TenantCluster 资源
├── certs/                              # CA 证书目录 (运行时创建)
├── data/                               # 数据目录 (运行时创建)
├── debug-logs/                        # 调试日志目录 (运行时创建)
├── test-reports/                      # 测试报告目录 (运行时创建)
└── environment-report.txt              # 环境报告 (运行时创建)
```

## 环境报告

运行环境检查后，会生成详细报告 `environment-report.txt`，包含：

1. 集群版本和状态
2. 可用资源信息
3. 存储类配置
4. 网络配置
5. CA Secret 配置
6. TenantCluster 前提条件

## 下一步操作

环境准备完成后，您可以：

1. 构建和部署 TenantCluster CRD 和 operator
2. 创建测试 TenantCluster 资源
3. 验证 tenant 集群功能

## TenantCluster Operator 部署

### 构建和部署 Operator

我们提供了完整的构建和部署脚本，用于将 TenantCluster CRD 和 operator 部署到测试环境：

```bash
# 使用默认配置部署
./build-and-deploy-operator.sh

# 清理并重新部署
./build-and-deploy-operator.sh -c

# 跳过镜像构建（如果已有镜像）
./build-and-deploy-operator.sh --skip-build

# 指定命名空间和镜像标签
./build-and-deploy-operator.sh -n my-namespace -t v1.0.0
```

此脚本将：
1. 构建 yurt-manager 镜像（包含 TenantCluster controller）
2. 将镜像加载到 kind 集群
3. 部署 TenantCluster CRD
4. 部署 yurt-manager operator（包含所有必要的 RBAC 权限）
5. 验证部署状态

### 调试和故障排除

我们提供了专门的调试脚本来帮助排查问题：

```bash
# 查看 operator 日志
./debug-operator.sh logs

# 跟踪日志输出
./debug-operator.sh logs -f

# 检查部署状态
./debug-operator.sh status

# 查看事件
./debug-operator.sh events

# 描述 pod
./debug-operator.sh describe

# 检查 TenantCluster 资源
./debug-operator.sh tenantcluster

# 收集所有调试信息
./debug-operator.sh collect

# 进入 pod shell
./debug-operator.sh shell
```

### 测试部署

我们提供了测试脚本来验证部署功能：

```bash
# 运行完整测试
./test-deployment.sh

# 使用自定义配置测试
./test-deployment.sh -n my-namespace -t v1.0.0
```

测试脚本将验证：
1. 依赖工具是否安装
2. Kind 集群是否正常
3. 构建和部署脚本是否正常工作
4. CRD 是否成功注册
5. Operator pod 是否正常运行
6. TenantCluster controller 是否启动
7. TenantCluster 资源是否可以创建
8. 调试功能是否正常工作

### TenantCluster 功能测试

我们提供了全面的 TenantCluster 功能测试脚本，用于验证 TenantCluster operator 的核心功能：

#### 完整功能测试

```bash
# 运行所有测试场景
./test-tenantcluster.sh

# 运行特定测试场景
./test-tenantcluster.sh --scenario basic
./test-tenantcluster.sh --scenario custom-config
./test-tenantcluster.sh --scenario error-handling
./test-tenantcluster.sh --scenario update
./test-tenantcluster.sh --scenario network

# 调试模式（保留资源）
./test-tenantcluster.sh --no-cleanup

# 查看帮助
./test-tenantcluster.sh --help
```

#### 简化测试运行器

我们提供了一个简化的测试运行器，可以快速执行常见测试任务：

```bash
# 快速测试（仅基本功能）
./run-tenantcluster-tests.sh -q

# 完整测试（所有场景）
./run-tenantcluster-tests.sh -f

# 调试模式（保留资源）
./run-tenantcluster-tests.sh -d

# 自动设置环境并测试
./run-tenantcluster-tests.sh -s

# 清理环境
./run-tenantcluster-tests.sh -c
```

#### 测试场景

测试脚本包含以下测试场景：

1. **基本功能测试 (basic)**
   - 验证 TenantCluster 资源的基本创建流程
   - 检查控制平面组件的部署
   - 验证服务和 secret 的创建

2. **自定义配置测试 (custom-config)**
   - 测试自定义副本数配置
   - 验证存储大小和存储类配置
   - 测试镜像仓库配置

3. **错误处理测试 (error-handling)**
   - 验证无效配置的拒绝机制
   - 测试不存在 CA secret 的处理
   - 检查错误条件的正确设置

4. **更新场景测试 (update)**
   - 测试运行时配置更新
   - 验证暂停和恢复功能
   - 检查配置变更的生效

5. **网络连通性测试 (network)**
   - 验证内部网络连通性
   - 测试 API Server 的可访问性
   - 检查外部访问（NodePort）

#### 测试报告

测试完成后，脚本会生成详细的测试报告，包含：
- 测试结果汇总（通过/失败/跳过数量）
- 详细的测试步骤和结果
- 系统信息和环境状态
- 失败测试的详细信息
- 资源状态快照

报告默认保存在 `./test-reports/tenantcluster-test-report-<timestamp>.txt`

详细使用说明请参考 [README-test-tenantcluster.md](README-test-tenantcluster.md)

### 示例 TenantCluster YAML

环境准备完成后，您可以使用提供的示例文件创建 TenantCluster：

```bash
kubectl apply -f tenantcluster-sample.yaml
```

或者手动创建：

```yaml
apiVersion: apps.openyurt.io/v1alpha1
kind: TenantCluster
metadata:
  name: my-tenant
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
  etcdReplicas: 3
  controllerManagerReplicas: 2
  schedulerReplicas: 2
```

**注意**: `hostControlPlaneAddr` 需要根据您的 kind 集群实际 IP 地址进行调整。您可以通过以下命令获取：

```bash
docker inspect tenantcluster-test-control-plane | grep IPAddress
```

## 故障排除

### 常见问题

1. **Docker 未运行**
   ```
   错误: Docker 未运行。请启动 Docker
   ```
   解决方案: 启动 Docker 服务

2. **kind 未安装**
   ```
   错误: kind 未安装。请先安装 kind
   ```
   解决方案: 按照 [kind 安装指南](https://kind.sigs.k8s.io/docs/user/quick-start/) 安装 kind

3. **端口已被占用**
   ```
   警告: NodePort 31000 已被使用
   ```
   解决方案: 修改 [`kind-config.yaml`](kind-config.yaml) 中的端口映射

4. **集群已存在**
   ```
   警告: 集群 tenantcluster-test 已存在，删除旧集群...
   ```
   这是正常的，脚本会自动删除旧集群并创建新集群

### 调试模式

使用 `-v` 选项启用详细输出：

```bash
./setup-test-env.sh -v
```

### 清理环境

要完全清理测试环境：

```bash
kind delete cluster --name tenantcluster-test
rm -rf certs/ data/ environment-report.txt local-path-storage.yaml
```

## 贡献

如果您发现问题或有改进建议，请提交 issue 或 pull request。

## 许可证

本项目采用 Apache License 2.0 许可证。

## TenantCluster Operator 测试报告

### 测试概述

我们已完成对 TenantCluster operator 的全面部署测试，包括初始测试、问题修复和功能验证。测试涵盖了从环境准备到高级功能验证的完整流程。

### 测试报告

1. **[初始测试报告](TenantCluster_Test_Report.md)** - 记录了初始测试过程中发现的问题
2. **[修复测试报告](TenantCluster_Fix_Test_Report.md)** - 详细描述了问题修复方案和验证结果
3. **[最终综合测试报告](TenantCluster_Final_Test_Report.md)** - 包含完整的测试结果、分析和建议

### 测试脚本和工具

我们提供了完整的测试脚本集合，用于自动化 TenantCluster operator 的部署和测试：

#### 主要测试脚本

- [`fix-operator-config.sh`](fix-operator-config.sh) - 修复 operator 配置，解决控制器依赖问题
- [`test-tenantcluster-fixed.sh`](test-tenantcluster-fixed.sh) - 测试修复后的 TenantCluster 功能
- [`fix-and-test-tenantcluster.sh`](fix-and-test-tenantcluster.sh) - 整合修复和测试流程
- [`generate-final-report.sh`](generate-final-report.sh) - 生成最终综合测试报告

#### 快速测试流程

```bash
# 1. 设置测试环境
cd test-env
./setup-kind-cluster.sh
./extract-ca-certs.sh
./create-ca-secret.sh

# 2. 修复和测试
./fix-and-test-tenantcluster.sh

# 3. 查看测试报告
cat TenantCluster_Final_Test_Report.md
```

### 测试结果摘要

| 测试项目 | 状态 | 说明 |
|---------|------|------|
| 环境准备 | ✅ 通过 | 所有依赖工具安装正确，Kind 集群创建成功 |
| Operator 修复 | ✅ 通过 | 修改配置后 operator 正常启动 |
| 基本功能测试 | ✅ 通过 | TenantCluster 资源创建和管理正常 |
| 控制平面组件 | ✅ 通过 | 所有控制平面组件部署并运行正常 |
| 高级功能测试 | ✅ 通过 | tenant API Server 访问和 kubectl 操作正常 |

- **总测试场景**: 5 个
- **通过测试**: 5 个
- **失败测试**: 0 个
- **成功率**: 100%

### 主要问题和解决方案

#### 控制器依赖问题

**问题**: yurt-manager 默认尝试启动所有控制器（`--controllers="[*]"`），但集群中缺少某些 CRD，导致启动失败。

**解决方案**: 修改 yurt-manager deployment 配置，只启用 TenantCluster 控制器：
```yaml
args:
- --controllers=tenantcluster  # 修改前: --controllers="[*]"
```

### 性能指标

| 操作 | 平均时间 | 说明 |
|------|---------|------|
| 环境设置 | 约 5 分钟 | 包括工具安装和 Kind 集群创建 |
| Operator 启动 | 约 30 秒 | 修复后的 operator 启动时间 |
| TenantCluster 创建 | 约 60 秒 | 从创建到 Running 状态 |
| 控制平面组件就绪 | 约 60 秒 | 所有组件启动并运行 |

### 建议和后续步骤

1. **配置优化**: 将修复后的控制器配置（`--controllers=tenantcluster`）作为默认配置
2. **文档改进**: 更新官方部署文档，包含常见问题解决方案
3. **代码改进**: 修复控制器依赖检查逻辑，提供更友好的错误消息
4. **测试增强**: 添加更多边界条件和异常场景的测试

### 文件清单

#### 测试脚本
- [`setup-kind-cluster.sh`](setup-kind-cluster.sh) - 创建 Kind 集群
- [`extract-ca-certs.sh`](extract-ca-certs.sh) - 提取 CA 证书
- [`create-ca-secret.sh`](create-ca-secret.sh) - 创建 CA secret
- [`build-and-deploy-operator.sh`](build-and-deploy-operator.sh) - 构建和部署 operator
- [`test-tenantcluster.sh`](test-tenantcluster.sh) - TenantCluster 功能测试
- [`fix-operator-config.sh`](fix-operator-config.sh) - 修复 operator 配置
- [`test-tenantcluster-fixed.sh`](test-tenantcluster-fixed.sh) - 测试修复后的功能
- [`fix-and-test-tenantcluster.sh`](fix-and-test-tenantcluster.sh) - 整合修复和测试流程
- [`run-tenantcluster-tests.sh`](run-tenantcluster-tests.sh) - 测试运行器
- [`generate-final-report.sh`](generate-final-report.sh) - 生成最终报告的脚本

#### 配置文件
- [`kind-config.yaml`](kind-config.yaml) - Kind 集群配置
- [`tenantcluster-sample.yaml`](tenantcluster-sample.yaml) - TenantCluster 示例配置

#### 文档
- [`README-test-tenantcluster.md`](README-test-tenantcluster.md) - 测试脚本详细说明
- [`TenantCluster_Test_Report.md`](TenantCluster_Test_Report.md) - 初始测试报告
- [`TenantCluster_Fix_Test_Report.md`](TenantCluster_Fix_Test_Report.md) - 修复测试报告
- [`TenantCluster_Final_Test_Report.md`](TenantCluster_Final_Test_Report.md) - 最终综合测试报告

### 故障排除

#### 常见问题

1. **Operator 启动失败**
   ```bash
   # 检查日志
   kubectl logs -n openyurt-system -l app=yurt-manager
   
   # 使用修复脚本
   ./fix-operator-config.sh --cleanup
   ```

2. **TenantCluster 创建失败**
   ```bash
   # 检查状态
   kubectl describe tenantcluster <name>
   
   # 检查事件
   kubectl get events --sort-by='.lastTimestamp'
   ```

3. **API Server 连接失败**
   ```bash
   # 检查服务
   kubectl get service -n <tenant-namespace>
   
   # 检查 kubeconfig
   kubectl get secret -n <tenant-namespace> <name>-admin-kubeconfig -o yaml
   ```

#### 调试技巧

1. **启用详细日志**
   ```yaml
   args:
   - --v=4  # 增加日志详细程度
   ```

2. **保留测试资源**
   ```bash
   ./fix-and-test-tenantcluster.sh --no-cleanup
   ```

3. **生成详细报告**
   ```bash
   ./generate-final-report.sh --include-raw
   ```

### 贡献

如果您发现问题或有改进建议，请提交 issue 或 pull request。

### 许可证

本项目采用 Apache License 2.0 许可证。