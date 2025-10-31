#!/bin/bash

# Copy Trade Bot 国际化部署脚本
# 用于快速部署国际化功能到现有系统

set -e

echo "=========================================="
echo "Copy Trade Bot 国际化部署脚本"
echo "=========================================="
echo ""

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 项目路径
PROJECT_DIR="/usr/local/server/copy-trade-bot/web"
STATIC_DIR="$PROJECT_DIR/static"
BACKUP_DIR="$PROJECT_DIR/backup_$(date +%Y%m%d_%H%M%S)"

# 检查是否在正确的目录
if [ ! -d "$PROJECT_DIR" ]; then
    echo -e "${RED}错误: 找不到项目目录 $PROJECT_DIR${NC}"
    exit 1
fi

echo -e "${YELLOW}步骤 1: 创建备份目录${NC}"
mkdir -p "$BACKUP_DIR"
echo -e "${GREEN}✓ 备份目录已创建: $BACKUP_DIR${NC}"
echo ""

echo -e "${YELLOW}步骤 2: 备份现有文件${NC}"
if [ -f "$STATIC_DIR/login.html" ]; then
    cp "$STATIC_DIR/login.html" "$BACKUP_DIR/login.html.bak"
    echo -e "${GREEN}✓ 已备份 login.html${NC}"
fi

if [ -f "$STATIC_DIR/index.html" ]; then
    cp "$STATIC_DIR/index.html" "$BACKUP_DIR/index.html.bak"
    echo -e "${GREEN}✓ 已备份 index.html${NC}"
fi

if [ -f "$STATIC_DIR/dashboard.html" ]; then
    cp "$STATIC_DIR/dashboard.html" "$BACKUP_DIR/dashboard.html.bak"
    echo -e "${GREEN}✓ 已备份 dashboard.html${NC}"
fi
echo ""

echo -e "${YELLOW}步骤 3: 检查 i18n.js 文件${NC}"
if [ ! -f "$STATIC_DIR/js/i18n.js" ]; then
    echo -e "${RED}错误: 找不到 i18n.js 文件${NC}"
    echo "请确保 i18n.js 已经创建在 $STATIC_DIR/js/ 目录下"
    exit 1
fi
echo -e "${GREEN}✓ i18n.js 文件存在${NC}"
echo ""

echo -e "${YELLOW}步骤 4: 部署选项${NC}"
echo "请选择部署方式:"
echo "1) 仅部署登录页面 (login.html)"
echo "2) 部署所有页面 (login.html + index.html + dashboard.html)"
echo "3) 仅查看文件状态，不进行部署"
echo "4) 退出"
echo ""
read -p "请输入选项 (1-4): " choice

case $choice in
    1)
        echo ""
        echo -e "${YELLOW}部署登录页面...${NC}"
        if [ -f "$STATIC_DIR/login_i18n.html" ]; then
            cp "$STATIC_DIR/login_i18n.html" "$STATIC_DIR/login.html"
            echo -e "${GREEN}✓ 登录页面已更新${NC}"
        else
            echo -e "${RED}错误: 找不到 login_i18n.html${NC}"
            exit 1
        fi
        ;;
    2)
        echo ""
        echo -e "${YELLOW}部署所有页面...${NC}"
        echo -e "${YELLOW}注意: index.html 和 dashboard.html 需要手动更新${NC}"
        echo "请参考 I18N_IMPLEMENTATION.md 文档进行手动更新"
        
        if [ -f "$STATIC_DIR/login_i18n.html" ]; then
            cp "$STATIC_DIR/login_i18n.html" "$STATIC_DIR/login.html"
            echo -e "${GREEN}✓ 登录页面已更新${NC}"
        fi
        ;;
    3)
        echo ""
        echo -e "${YELLOW}文件状态检查:${NC}"
        echo ""
        echo "i18n 核心文件:"
        [ -f "$STATIC_DIR/js/i18n.js" ] && echo -e "${GREEN}✓${NC} i18n.js" || echo -e "${RED}✗${NC} i18n.js"
        echo ""
        echo "示例文件:"
        [ -f "$STATIC_DIR/login_i18n.html" ] && echo -e "${GREEN}✓${NC} login_i18n.html" || echo -e "${RED}✗${NC} login_i18n.html"
        echo ""
        echo "现有页面:"
        [ -f "$STATIC_DIR/login.html" ] && echo -e "${GREEN}✓${NC} login.html" || echo -e "${RED}✗${NC} login.html"
        [ -f "$STATIC_DIR/index.html" ] && echo -e "${GREEN}✓${NC} index.html" || echo -e "${RED}✗${NC} index.html"
        [ -f "$STATIC_DIR/dashboard.html" ] && echo -e "${GREEN}✓${NC} dashboard.html" || echo -e "${RED}✗${NC} dashboard.html"
        echo ""
        echo "文档文件:"
        [ -f "$PROJECT_DIR/I18N_README.md" ] && echo -e "${GREEN}✓${NC} I18N_README.md" || echo -e "${RED}✗${NC} I18N_README.md"
        [ -f "$PROJECT_DIR/I18N_IMPLEMENTATION.md" ] && echo -e "${GREEN}✓${NC} I18N_IMPLEMENTATION.md" || echo -e "${RED}✗${NC} I18N_IMPLEMENTATION.md"
        [ -f "$PROJECT_DIR/I18N_SUMMARY.md" ] && echo -e "${GREEN}✓${NC} I18N_SUMMARY.md" || echo -e "${RED}✗${NC} I18N_SUMMARY.md"
        echo ""
        exit 0
        ;;
    4)
        echo ""
        echo "退出部署"
        exit 0
        ;;
    *)
        echo -e "${RED}无效的选项${NC}"
        exit 1
        ;;
esac

echo ""
echo -e "${YELLOW}步骤 5: 验证部署${NC}"
if [ -f "$STATIC_DIR/login.html" ]; then
    if grep -q "data-i18n" "$STATIC_DIR/login.html"; then
        echo -e "${GREEN}✓ login.html 包含 i18n 标记${NC}"
    else
        echo -e "${YELLOW}⚠ login.html 可能未正确更新${NC}"
    fi
fi
echo ""

echo "=========================================="
echo -e "${GREEN}部署完成！${NC}"
echo "=========================================="
echo ""
echo "备份文件位置: $BACKUP_DIR"
echo ""
echo "下一步:"
echo "1. 重启 Web 服务器"
echo "2. 访问登录页面测试语言切换功能"
echo "3. 查看 I18N_README.md 了解更多使用方法"
echo ""
echo "如需回滚，请运行:"
echo "  cp $BACKUP_DIR/login.html.bak $STATIC_DIR/login.html"
echo ""

# 询问是否重启服务
read -p "是否需要重启 Web 服务? (y/n): " restart
if [ "$restart" = "y" ] || [ "$restart" = "Y" ]; then
    echo ""
    echo -e "${YELLOW}正在查找 Web 服务进程...${NC}"
    
    # 查找 Go 进程
    if pgrep -f "copytrade" > /dev/null; then
        echo -e "${YELLOW}找到 Copy Trade Bot 进程${NC}"
        read -p "确认重启服务? (y/n): " confirm
        if [ "$confirm" = "y" ] || [ "$confirm" = "Y" ]; then
            echo "请手动重启服务"
            echo "提示: 可以使用 systemctl restart copytrade 或其他方式"
        fi
    else
        echo -e "${YELLOW}未找到运行中的服务${NC}"
        echo "请手动启动服务"
    fi
fi

echo ""
echo -e "${GREEN}感谢使用 Copy Trade Bot 国际化部署脚本！${NC}"
