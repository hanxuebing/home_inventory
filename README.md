# 家庭物品整理收纳系统

移动端优先的家庭物品管理 Web 应用：家庭成员录入物品（名称 / 数量 / 图片 / 备注 / 分类），
家庭内共享可见，支持模糊搜索与分类筛选；内置 JWT 双 Token 认证与 RBAC 三级角色权限。
依据 [登录鉴权系统设计稿 v1.0](../study/auth-system-design/登录鉴权系统设计稿.html) 落地。

## 快速开始

```powershell
# 0) 隧道（MySQL/Redis 在服务器上，映射到本机 3306/6379）
ssh -N study-db        # ssh config 配置见 docs/开发调试指南.md

# 1) 初始化数据库（首次；连 127.0.0.1:3306 执行）
mysql -h 127.0.0.1 -u root -p < sql\init.sql

# 2) 后端
cd server
Copy-Item .env.example .env     # 填 MySQL 密码 / Redis 密码 / JWT_SECRET
go run ./cmd/api                # :8080

# 3) 前端（另开终端）
cd web
npm install
npm run dev                     # http://localhost:5173（手机调试加 -- --host）
```

## 种子账号

| 账号 | 密码 | 角色 | 能看到的功能 |
|------|------|------|------------|
| admin | Admin@123456 | 超级管理员 | 全部：家庭管理、用户管理、审计日志、全部物品 |
| zhangsan | Family@123456 | 家庭管理员（张家） | 家庭物品全权、成员管理、移交管理员、分类管理 |
| zhangmei | Member@123456 | 普通成员（张家） | 查看家庭物品、录入、仅改删自己负责的、分类只读 |
| lisi / liwang | Family@123456 / Member@123456 | 李家管理员 / 成员 | 同上（李家） |

> 登录后所有功能差别由角色自动体现，无需切换系统。

## 目录

```
sql/init.sql        建库建表 + 权限码/角色/家庭/物品种子（Argon2id 哈希已内置）
server/             Go Gin 单服务（:8080）：认证 + 物品业务 + 管理
web/                Vue3 + TS + Element Plus 移动端 SPA（:5173）
docs/               项目系统文档 · 开发调试指南
.vscode/            断点调试（Go delve + Chrome）与构建任务
```

## 核心机制（详见 docs/项目系统文档.md）

- **认证**：access(15m, 内存) + refresh(7d, httpOnly Cookie) 双 Token，一次一换 +
  重用检测（旧 refresh 二次出现 → 撤销整个 token family）
- **授权**：RBAC 权限码中间件 `RequirePermission(code)` + handler 行级校验
  （家庭隔离 / owner 判定），前端动态路由 + v-permission 只管体验
- **业务规则**：物品家庭内共享；member 只能改删自己负责的；删除用户必须指定
  物品接收人，责任整体移交且历史全程留痕（CREATE/UPDATE/DELETE/TRANSFER）

## 常用命令

```powershell
cd server
go run ./cmd/hashpw                    # 交互式生成 Argon2id 哈希（改种子密码用）
go build ./...                         # 编译检查
cd ../web
npm run build                          # 生产构建（含 TS 类型检查）
```

VSCode 断点调试（后端 delve / 前端 Chrome / 前后端联调）见 `docs/开发调试指南.md`。
