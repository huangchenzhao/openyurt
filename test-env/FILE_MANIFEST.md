# TenantCluster 测试文件清单和使用说明

## 文件分类

### 1. 环境设置脚本

| 文件名 | 描述 | 大小 | 修改时间 | 依赖关系 |
|--------|------|------|---------|----------|
| [`install-tools.sh`](install-tools.sh) | 安装必要的工具（kubectl, kind, docker） | ~2KB | 2025-12-21 | 无 |
| [`setup-kind-cluster.sh`](setup-kind-cluster.sh) | 创建 Kind 集群用于测试 | ~3.5KB | 2025-12-21 | install-tools.sh |
| [`extract-ca-certs.sh`](extract-ca-certs.sh) | 从 Kind 集群提取 CA 证书 | ~2KB | 2025-12-21 | setup-kind-cluster.sh |
| [`create-ca-secret.sh`](create-ca-secret.sh) | 创建 host-k8s-ca secret | ~2KB | 2025-12-21 | extract-ca-certs.sh |

### 2. Operator 部署脚本

| 文件名 | 描述 | 大小 | 修改时间 | 依赖关系 |
|--------|------|------|---------|----------|
| [`build-and-deploy-operator.sh`](build-and-deploy-operator.sh) | 构建和部署 TenantCluster operator | ~4.5KB | 2025-12-21 | 环境设置脚本 |
| [`fix-operator-config.sh`](fix-operator-config.sh) | 修复 operator 配置，解决控制器依赖问题 | ~5.5KB | 2025-12-21 | build-and-deploy-operator.sh |

### 3. 测试脚本

| 文件名 | 描述 | 大小 | 修改时间 | 依赖关系 |
|--------|------|------|---------|----------|
| [`test-tenantcluster.sh`](test-tenantcluster.sh) | TenantCluster 功能测试脚本 | ~9.8KB | 2025-12-21 | fix-operator-config.sh |
| [`test-tenantcluster-fixed.sh`](test-tenantcluster-fixed.sh) | 测试修复后的 TenantCluster 功能 | ~6KB | 2025-12-21 | fix-operator-config.sh |
| [`test-deployment.sh`](test-deployment.sh) | 测试部署脚本功能 | ~3KB | 2025-12-21 | build-and-deploy-operator.sh |

### 4. 综合脚本

| 文件名 | 描述 | 大小 | 修改时间 | 依赖关系 |
|--------|------|------|---------|----------|
| [`fix-and-test-tenantcluster.sh`](fix-and-test-tenantcluster.sh) | 整合修复和测试流程 | ~3.3KB | 2025-12-21 | fix-operator-config.sh, test-tenantcluster-fixed.sh |
| [`run-tenantcluster-tests.sh`](run-tenantcluster-tests.sh) | 测试运行器（简化版） | ~3KB | 2025-12-21 | 所有测试脚本 |
| [`simulated-fix-and-test.sh`](simulated-fix-and-test.sh) | 模拟完整的修复和测试流程 | ~3.2KB | 2025-12-21 | 无 |
| [`generate-final-report.sh`](generate-final-report.sh) | 生成最终综合测试报告 | ~8KB | 2025-12-21 | 所有测试脚本和报告 |

### 5. 调试和辅助脚本

| 文件名 | 描述 | 大小 | 修改时间 | 依赖关系 |
|--------|------|------|---------|----------|
| [`debug-operator.sh`](debug-operator.sh) | 调试和故障排除工具 | ~4KB | 2025-12-21 | build-and-deploy-operator.sh |
| [`test-scripts.sh`](test-scripts.sh) | 测试脚本功能 | ~2KB | 2025-12-21 | 所有脚本 |

### 6. 配置文件

| 文件名 | 描述 | 大小 | 修改时间 | 用途 |
|--------|------|------|---------|------|
| [`kind-config.yaml`](kind-config.yaml) | Kind 集群配置 | ~1KB | 2025-12-21 | setup-kind-cluster.sh |
| [`tenantcluster-sample.yaml`](tenantcluster-sample.yaml) | TenantCluster 示例配置 | ~1.2KB | 2025-12-21 | test-tenantcluster.sh |

### 7. 文档文件

| 文件名 | 描述 | 大小 | 修改时间 | 用途 |
|--------|------|------|---------|------|
| [`README.md`](README.md) | 测试环境概述和使用说明 | ~15KB | 2025-12-21 | 项目主文档 |
| [`README-test-tenantcluster.md`](README-test-tenantcluster.md) | 测试脚本详细说明 | ~7KB | 2025-12-21 | 测试脚本文档 |
| [`TenantCluster_Test_Report.md`](TenantCluster_Test_Report.md) | 初始测试报告 | ~8KB | 2025-12-21 | 测试结果记录 |
| [`TenantCluster_Fix_Test_Report.md`](TenantCluster_Fix_Test_Report.md) | 修复测试报告 | ~12KB | 2025-12-21 | 修复方案记录 |
| [`TenantCluster_Final_Test_Report.md`](TenantCluster_Final_Test_Report.md) | 最终综合测试报告 | ~25KB | 2025-12-21 | 完整测试总结 |
| [`FILE_MANIFEST.md`](FILE_MANIFEST.md) | 本文件清单和使用说明 | ~8KB | 2025-12-21 | 文件索引和使用指南 |

## 使用说明

### 快速开始

```bash
# 1. 进入测试目录
cd test-env

# 2. 一键设置环境并测试
./fix-and-test-tenantcluster.sh

# 3. 查看测试报告
cat TenantCluster_Final_Test_Report.md
```

### 分步执行

#### 步骤 1: 环境准备

```bash
# 安装必要工具
./install-tools.sh

# 创建 Kind 集群
./setup-kind-cluster.sh

# 提取 CA 证书
./extract-ca-certs.sh

# 创建 CA secret
./create-ca-secret.sh
```

#### 步骤 2: Operator 部署

```bash
# 构建和部署 operator（可能失败）
./build-and-deploy-operator.sh

# 修复 operator 配置
./fix-operator-config.sh --cleanup
```

#### 步骤 3: 功能测试

```bash
# 测试修复后的功能
./test-tenantcluster-fixed.sh

# 或者运行完整测试套件
./test-tenantcluster.sh
```

#### 步骤 4: 报告生成

```bash
# 生成最终综合报告
./generate-final-report.sh

# 包含原始报告内容
./generate-final-report.sh --include-raw
```

### 高级用法

#### 自定义测试场景

```bash
# 只运行基本功能测试
./test-tenantcluster.sh --scenario basic

# 运行自定义配置测试
./test-tenantcluster.sh --scenario custom-config

# 运行错误处理测试
./test-tenantcluster.sh --scenario error-handling

# 运行更新场景测试
./test-tenantcluster.sh --scenario update

# 运行网络连通性测试
./test-tenantcluster.sh --scenario network
```

#### 调试模式

```bash
# 保留测试资源用于调试
./fix-and-test-tenantcluster.sh --no-cleanup

# 跳过镜像构建
./fix-operator-config.sh --skip-build

# 启用详细日志
./test-tenantcluster.sh --timeout 1200

# 查看调试信息
./debug-operator.sh logs
./debug-operator.sh status
./debug-operator.sh events
```

#### 自定义配置

```bash
# 使用自定义集群名称
KIND_CLUSTER_NAME=my-cluster ./setup-kind-cluster.sh

# 使用自定义命名空间
NAMESPACE=my-namespace ./fix-operator-config.sh

# 使用自定义镜像
IMAGE_REPO=my-repo IMAGE_TAG=v1.0.0 ./fix-operator-config.sh

# 指定报告目录
REPORT_DIR=/tmp/reports ./generate-final-report.sh
```

### 脚本依赖关系图

```
install-tools.sh
    |
    v
setup-kind-cluster.sh
    |
    v
extract-ca-certs.sh
    |
    v
create-ca-secret.sh
    |
    v
build-and-deploy-operator.sh
    |
    v
fix-operator-config.sh
    |
    +-------------------+
    |                   |
    v                   v
test-tenantcluster.sh  test-tenantcluster-fixed.sh
    |                   |
    +-------------------+
    |
    v
fix-and-test-tenantcluster.sh
    |
    v
generate-final-report.sh
```

### 目录结构

```
test-env/
├── README.md                           # 项目主文档
├── README-test-tenantcluster.md        # 测试脚本详细说明
├── FILE_MANIFEST.md                   # 文件清单和使用说明（本文件）
├── install-tools.sh                   # 安装必要工具
├── setup-kind-cluster.sh              # 创建 Kind 集群
├── extract-ca-certs.sh                # 提取 CA 证书
├── create-ca-secret.sh                # 创建 CA secret
├── build-and-deploy-operator.sh       # 构建和部署 operator
├── fix-operator-config.sh             # 修复 operator 配置
├── test-tenantcluster.sh             # TenantCluster 功能测试
├── test-tenantcluster-fixed.sh       # 测试修复后的功能
├── test-deployment.sh                # 测试部署脚本功能
├── fix-and-test-tenantcluster.sh     # 整合修复和测试流程
├── run-tenantcluster-tests.sh        # 测试运行器
├── simulated-fix-and-test.sh         # 模拟修复和测试流程
├── generate-final-report.sh          # 生成最终报告
├── debug-operator.sh                 # 调试和故障排除工具
├── test-scripts.sh                  # 测试脚本功能
├── kind-config.yaml                 # Kind 集群配置
├── tenantcluster-sample.yaml         # TenantCluster 示例配置
├── TenantCluster_Test_Report.md      # 初始测试报告
├── TenantCluster_Fix_Test_Report.md  # 修复测试报告
├── TenantCluster_Final_Test_Report.md # 最终综合测试报告
├── certs/                          # CA 证书目录（运行时创建）
├── data/                           # 数据目录（运行时创建）
├── debug-logs/                     # 调试日志目录（运行时创建）
└── test-reports/                   # 测试报告目录（运行时创建）
```

### 最佳实践

1. **顺序执行**: 按照依赖关系顺序执行脚本，确保环境正确设置
2. **错误处理**: 脚本内置了错误处理，但建议在执行前检查环境
3. **资源清理**: 正常情况下让脚本自动清理资源，调试时使用 `--no-cleanup`
4. **日志收集**: 使用 `--v=4` 参数启用详细日志，便于问题排查
5. **版本控制**: 建议在执行前提交代码变更，便于回滚

### 常见问题

1. **权限问题**: 确保脚本有执行权限 `chmod +x *.sh`
2. **依赖缺失**: 运行 `install-tools.sh` 安装必要工具
3. **端口冲突**: 修改 `kind-config.yaml` 中的端口映射
4. **资源不足**: 增加 Docker 或 Kubernetes 资源限制
5. **网络问题**: 检查防火墙和代理设置

---

**文档更新时间**: 2025-12-21T10:04:00Z
**版本**: TenantCluster v1alpha1