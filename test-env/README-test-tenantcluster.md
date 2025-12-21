# TenantCluster 测试脚本使用指南

## 概述

`test-tenantcluster.sh` 是一个全面的测试脚本，用于验证 TenantCluster operator 的核心功能。该脚本包含多种测试场景，可以验证 TenantCluster 资源的创建、配置、更新和错误处理等功能。

## 功能特性

### 测试场景

1. **基本功能测试 (basic)**
   - 验证 TenantCluster 资源的基本创建流程
   - 检查控制平面组件（etcd、apiserver、scheduler、controller-manager）的部署
   - 验证服务、命名空间和 kubeconfig secret 的创建

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

### 核心功能

- **完整的验证逻辑**：检查所有控制平面组件的部署状态
- **详细的日志输出**：提供清晰的测试步骤和结果
- **超时机制**：防止测试无限等待
- **资源清理**：测试完成后自动清理资源
- **测试报告**：生成详细的测试报告
- **错误处理**：完善的错误处理和恢复机制

## 使用方法

### 前提条件

1. 已安装 kubectl、docker 和 kind
2. 已创建 Kind 集群（默认名称：tenantcluster-test）
3. 已部署 TenantCluster CRD 和 operator
4. 已创建 host-k8s-ca secret

### 基本用法

```bash
# 运行所有测试
./test-env/test-tenantcluster.sh

# 运行特定测试场景
./test-env/test-tenantcluster.sh --scenario basic

# 查看帮助信息
./test-env/test-tenantcluster.sh --help
```

### 高级用法

```bash
# 指定集群和命名空间
./test-env/test-tenantcluster.sh --cluster my-cluster --namespace my-namespace

# 设置超时时间（默认 600 秒）
./test-env/test-tenantcluster.sh --timeout 1200

# 测试完成后不清理资源（用于调试）
./test-env/test-tenantcluster.sh --no-cleanup

# 指定报告目录
./test-env/test-tenantcluster.sh --report-dir ./my-reports

# 跳过 operator 部署检查
./test-env/test-tenantcluster.sh --skip-deployment
```

## 命令行选项

| 选项 | 描述 | 默认值 |
|------|------|--------|
| `-h, --help` | 显示帮助信息 | - |
| `-c, --cluster <name>` | Kind 集群名称 | tenantcluster-test |
| `-n, --namespace <namespace>` | 命名空间 | openyurt-system |
| `-t, --timeout <seconds>` | 测试超时时间 | 600 |
| `--no-cleanup` | 测试完成后不清理资源 | false |
| `--report-dir <dir>` | 报告目录 | ./test-reports |
| `--scenario <scenario>` | 运行特定测试场景 | all |
| `--skip-deployment` | 跳过 operator 部署检查 | false |

## 测试场景详解

### 1. 基本功能测试

验证 TenantCluster 的基本创建流程：

- 创建使用默认配置的 TenantCluster
- 检查命名空间是否正确创建
- 验证所有控制平面组件的部署
- 检查服务和 secret 的创建
- 验证 kubeconfig 的生成

### 2. 自定义配置测试

验证自定义配置的正确应用：

- 创建使用自定义配置的 TenantCluster
- 验证 etcd 副本数配置
- 检查 controller-manager 和 scheduler 副本数
- 验证存储大小配置
- 检查镜像仓库配置

### 3. 错误处理测试

验证错误情况的处理：

- 测试无效配置的拒绝
- 验证不存在 CA secret 的处理
- 检查超出范围值的处理
- 验证错误条件的设置

### 4. 更新场景测试

验证运行时配置更新：

- 创建 TenantCluster 并等待初始化
- 更新副本数和存储配置
- 验证更新的生效
- 测试暂停和恢复功能

### 5. 网络连通性测试

验证网络功能：

- 检查 API Server 服务的创建
- 测试内部网络连通性
- 验证外部访问（如果可能）
- 检查服务端点的正确性

## 测试报告

测试完成后，脚本会生成详细的测试报告，包含：

- 测试结果汇总（通过/失败/跳过数量）
- 详细的测试步骤和结果
- 系统信息和环境状态
- 失败测试的详细信息
- 资源状态快照

报告默认保存在 `./test-reports/tenantcluster-test-report-<timestamp>.txt`

## 故障排除

### 常见问题

1. **测试失败：Kind 集群不存在**
   ```bash
   # 创建测试集群
   ./test-env/setup-kind-cluster.sh
   ```

2. **测试失败：TenantCluster CRD 未安装**
   ```bash
   # 部署 CRD 和 operator
   ./test-env/build-and-deploy-operator.sh
   ```

3. **测试失败：host-k8s-ca secret 不存在**
   ```bash
   # 创建 CA secret
   ./test-env/create-ca-secret.sh
   ```

4. **测试超时**
   - 增加超时时间：`--timeout 1200`
   - 检查系统资源是否充足
   - 查看operator 日志：`kubectl logs -n openyurt-system -l app=yurt-manager`

### 调试技巧

1. **保留测试资源**
   ```bash
   ./test-env/test-tenantcluster.sh --no-cleanup
   ```

2. **查看详细日志**
   ```bash
   # 查看 operator 日志
   kubectl logs -n openyurt-system -l app=yurt-manager -f
   
   # 查看 TenantCluster 状态
   kubectl describe tenantcluster <name>
   ```

3. **检查组件状态**
   ```bash
   # 查看所有测试资源
   kubectl get tenantcluster,all --all-namespaces | grep tc-test
   ```

## 示例输出

```
[INFO] 开始 TenantCluster 功能测试...
[INFO] 初始化测试环境...
[SUCCESS] 测试环境初始化完成
[TEST] 开始基本功能测试...
[INFO] 创建基本 TenantCluster: tc-test-1640995200-basic
[SUCCESS] 基本功能测试通过
[TEST] 开始自定义配置测试...
[SUCCESS] 自定义配置测试通过
...
======================================
测试结果汇总
======================================
总测试数: 5
通过: 5
失败: 0
跳过: 0
成功率: 100%
```

## 最佳实践

1. **定期运行测试**：在代码变更后运行完整测试
2. **使用特定场景**：开发时运行相关测试场景
3. **检查报告**：仔细阅读测试报告了解系统状态
4. **清理资源**：正常情况下让脚本自动清理资源
5. **调试模式**：遇到问题时使用 `--no-cleanup` 保留资源

## 贡献

如果发现问题或有改进建议，请：

1. 查看测试报告了解详细信息
2. 检查相关日志
3. 提供复现步骤和环境信息
4. 提交 Issue 或 Pull Request