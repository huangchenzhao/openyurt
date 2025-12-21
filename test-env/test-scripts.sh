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

# Script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

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

# Test script syntax
test_script_syntax() {
    local script=$1
    info "检查脚本语法: $script"
    
    if bash -n "$SCRIPT_DIR/$script"; then
        success "$script 语法正确"
        return 0
    else
        error "$script 语法错误"
        return 1
    fi
}

# Test script help
test_script_help() {
    local script=$1
    local help_flag=$2
    info "测试帮助功能: $script $help_flag"
    
    if bash "$SCRIPT_DIR/$script" $help_flag &> /dev/null; then
        success "$script 帮助功能正常"
        return 0
    else
        warning "$script 帮助功能可能有问题"
        return 1
    fi
}

# Test file permissions
test_file_permissions() {
    local file=$1
    info "检查文件权限: $file"
    
    if [ -x "$SCRIPT_DIR/$file" ]; then
        success "$file 具有执行权限"
        return 0
    else
        error "$file 没有执行权限"
        return 1
    fi
}

# Test configuration files
test_config_files() {
    local file=$1
    info "检查配置文件: $file"
    
    if [ -f "$SCRIPT_DIR/$file" ]; then
        # Test YAML syntax if available
        if command -v python3 &> /dev/null; then
            if python3 -c "import yaml; yaml.safe_load(open('$SCRIPT_DIR/$file'))" 2>/dev/null; then
                success "$file YAML 语法正确"
            else
                warning "$file YAML 语法可能有问题"
            fi
        else
            success "$file 存在"
        fi
        return 0
    else
        error "$file 不存在"
        return 1
    fi
}

# Main test function
main() {
    info "开始测试 TenantCluster 测试环境脚本..."
    
    local all_tests_passed=true
    local failed_tests=()
    
    # Change to script directory
    cd "$SCRIPT_DIR"
    
    # Test file permissions
    info "测试文件权限..."
    local scripts=(
        "setup-test-env.sh"
        "install-tools.sh"
        "setup-kind-cluster.sh"
        "extract-ca-certs.sh"
        "create-ca-secret.sh"
        "install-local-path-provisioner.sh"
        "check-environment.sh"
    )
    
    for script in "${scripts[@]}"; do
        if ! test_file_permissions "$script"; then
            all_tests_passed=false
            failed_tests+=("$script (权限)")
        fi
    done
    
    # Test script syntax
    info "测试脚本语法..."
    for script in "${scripts[@]}"; do
        if ! test_script_syntax "$script"; then
            all_tests_passed=false
            failed_tests+=("$script (语法)")
        fi
    done
    
    # Test help functionality
    info "测试帮助功能..."
    if ! test_script_help "install-tools.sh" "--help"; then
        all_tests_passed=false
        failed_tests+=("install-tools.sh (帮助)")
    fi
    
    if ! test_script_help "setup-test-env.sh" "--help"; then
        all_tests_passed=false
        failed_tests+=("setup-test-env.sh (帮助)")
    fi
    
    # Test configuration files
    info "测试配置文件..."
    local config_files=(
        "kind-config.yaml"
        "tenantcluster-sample.yaml"
    )
    
    for config in "${config_files[@]}"; do
        if ! test_config_files "$config"; then
            all_tests_passed=false
            failed_tests+=("$config")
        fi
    fi
    
    # Test README
    info "测试 README..."
    if [ -f "$SCRIPT_DIR/README.md" ]; then
        success "README.md 存在"
    else
        all_tests_passed=false
        failed_tests+=("README.md")
    fi
    
    # Summary
    echo ""
    if [ "$all_tests_passed" = true ]; then
        success "所有测试通过！脚本和配置文件准备就绪。"
    else
        error "部分测试失败:"
        for failed_test in "${failed_tests[@]}"; do
            echo "  - $failed_test"
        done
    fi
    
    # Next steps
    echo ""
    info "下一步操作:"
    if [ "$all_tests_passed" = true ]; then
        echo "1. 运行 ./install-tools.sh 安装必要工具"
        echo "2. 运行 ./setup-test-env.sh 设置测试环境"
        echo "3. 部署 TenantCluster CRD 和 operator"
        echo "4. 使用 kubectl apply -f tenantcluster-sample.yaml 创建测试租户集群"
    else
        echo "1. 修复失败的测试"
        echo "2. 重新运行此测试脚本"
    fi
}

# Run main function
main "$@"