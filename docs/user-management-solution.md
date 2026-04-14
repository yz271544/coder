# Coder 用户管理方案

## 问题描述

Coder 在首次安装时可以在登录界面注册第一个管理员账号（称为 Initial User）。当第一个账号创建完成后，注册入口会消失，后续无法通过界面添加新用户。

当前场景：
- 已在 `enterprise/cli/server.go` 中添加了离线 license 加载逻辑
- 已在 `enterprise/cli/generate_license.go` 中添加了 `generate-license` 命令
- 需求：**仅管理员可以添加用户**

---

## Coder 现有用户机制分析

### 1. 首次用户创建（First User）

**API 端点**：`POST /api/v2/users/first`

**代码位置**：`coderd/users.go` - `postFirstUser` 函数

**核心逻辑**：
```go
// 检查是否已有用户存在
userCount, err := api.Database.GetUserCount(ctx, false)
if userCount != 0 {
    // 如果已有用户存在，返回冲突错误
    httpapi.Write(ctx, rw, http.StatusConflict, codersdk.Response{
        Message: "The initial user has already been created.",
    })
    return
}
```

**特点**：
- 仅在数据库中没有任何用户时可以调用
- 创建后自动授予 Owner 角色（超级管理员）
- 这是唯一可以绕过认证创建用户的接口

### 2. 普通用户创建

**API 端点**：`POST /api/v2/users`

**权限要求**：需要 Admin 或 Owner 角色

**认证方式**：需要有效的 Session Token

---

## 解决方案

### 方案一：使用 Admin API 创建用户（推荐）

**原理**：使用已创建的 Admin 账号，通过 Coder 内置的 Admin API 手动创建用户。

**使用方式**：

```bash
# 使用 coder CLI 创建用户
coder users create --email newuser@company.com --username newuser

# 查看所有用户
coder users list

# 更新用户角色
coder users update newuser --role admin
```

**通过 API 调用**：
```bash
# 首先获取 admin 的 API Token（在 Coder Web UI 的 Account -> Tokens 中生成）
export CODER_URL="https://coder.example.com"
export CODER_API_TOKEN="your-api-token"

# 创建用户
curl -X POST "$CODER_URL/api/v2/users" \
  -H "Authorization: Bearer $CODER_API_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "email": "newuser@company.com",
    "username": "newuser"
  }'
```

**优点**：
- 无需修改代码
- 无需额外部署系统
- 符合"仅管理员添加用户"的需求
- 易于审计和管控

**缺点**：
- 每次添加用户需要管理员手动操作

---

### 方案二：对接 Keycloak OIDC（企业级方案）

**原理**：配置 Coder 使用 OIDC 认证，用户通过 Keycloak 登录时自动在 Coder 中创建账号。

**前提条件**：
- 已有 Keycloak 环境
- 配置好 HTTPS（可使用自签名证书）

**Coder 配置**：
```bash
# 环境变量或配置文件
CODER_OIDC_CLIENT_ID=coder
CODER_OIDC_CLIENT_SECRET=your-client-secret
CODER_OIDC_ISSUER_URL=https://keycloak.example.com/realms/coder
CODER_OIDC_EMAIL_DOMAIN=yourcompany.com
```

**Keycloak 配置**：
1. 创建 Realm
2. 创建 Client（选择 OpenID Connect）
3. 配置 Valid Redirect URLs：`https://coder.example.com/oidc/callback`
4. 配置 Client Secret

**用户登录流程**：
1. 用户访问 Coder
2. Coder 重定向到 Keycloak 登录页面
3. 用户在 Keycloak 完成认证
4. Keycloak 回调 Coder，自动创建用户并建立关联

**优点**：
- 与企业身份管理集成
- 用户生命周期由 Keycloak 管理
- 支持 SSO（单点登录）

**缺点**：
- 需要部署和维护 Keycloak
- 需要配置 HTTPS
- 用户登录时自动创建账号，无法实现"仅管理员添加"

---

### 方案三：工程改造放开注册

**原理**：修改 Coder 源码，允许在第一个用户创建后仍可通过 API 注册新用户。

**修改位置**：`coderd/users.go` - `postFirstUser` 函数

**需要修改的部分**：删除第 197-203 行的 userCount 检查

```go
// 删除这段代码
if userCount != 0 {
    httpapi.Write(ctx, rw, http.StatusConflict, codersdk.Response{
        Message: "The initial user has already been created.",
    })
    return
}
```

**改造后的使用方式**：
- 任何人都可以调用 `POST /api/v2/users/first` 创建用户
- 第一个创建的用户为 Owner 角色
- 后续创建的用户为普通成员

**安全建议**（如果采用此方案）：
1. 添加注册验证码机制
2. 限制注册邮箱域名（如仅限公司域名）
3. 添加管理员审批流程
4. 记录详细的审计日志

**优点**：
- 实现用户自助注册
- 无需额外部署系统

**缺点**：
- 修改核心代码，未来合并上游困难
- 需要持续维护
- 不符合"仅管理员添加"的需求

---

## 方案对比

| 方案          | 代码改动     | 额外依赖      | 用户管控       | 实施难度     |
|---------------|-------------|--------------|--------------|------------|
| Admin API     | 无           | 无            | 管理员完全可控 | 简单         |
| Keycloak OIDC | 无           | 需要 Keycloak | Keycloak 管理  | 中等         |
| 工程改造      | 需要修改源码 | 无            | 放开注册       | 简单但需维护 |

---

## 推荐方案

根据你**仅管理员添加用户**的需求，推荐**方案一：使用 Admin API**。

理由：
1. 无需修改代码，便于维护
2. 完全符合"仅管理员添加"的管控需求
3. 可以集成到现有运维流程中

---

## 附录：批量创建用户脚本示例

```bash
#!/bin/bash
# create_users.sh - 批量创建用户脚本

CODER_URL="https://coder.example.com"
API_TOKEN="your-admin-api-token"

# 读取用户列表文件，每行格式：email,username
while IFS=',' read -r email username; do
    echo "Creating user: $username ($email)"
    
    response=$(curl -s -X POST "$CODER_URL/api/v2/users" \
        -H "Authorization: Bearer $API_TOKEN" \
        -H "Content-Type: application/json" \
        -d "{\"email\":\"$email\",\"username\":\"$username\"}")
    
    if echo "$response" | grep -q "id"; then
        echo "  ✓ User created successfully"
    else
        echo "  ✗ Failed: $response"
    fi
done < users.csv
```

使用方式：
```bash
# 创建 users.csv 文件
echo "john@company.com,john
jane@company.com,jane" > users.csv

# 执行脚本
chmod +x create_users.sh
./create_users.sh
```

---

## 附录二：离线 License 报错解决方案

### 问题描述

启动 Coder 时看到以下错误：
```
Failed to parse offline license
error= license has invalid or missing license_expires claim
```

登录后看到 License Issue 告警：
```
Invalid license parsing claims: license has invalid or missing license_expires claim
```

### 根因分析

查看代码 `enterprise/coderd/license/license.go` 第 706-709 行：

```go
yearsHardLimit := time.Now().Add(5 /* years */ * 365 * 24 * time.Hour)
if claims.LicenseExpires == nil || claims.LicenseExpires.Time.After(yearsHardLimit) {
    return nil, ErrMissingLicenseExpires
}
```

**Coder 验证规则**：`license_expires` 不能超过当前时间 + 5 年

而 `GenerateOfflineLicense` 函数生成的是 10 年 license：
```go
// enterprise/coderd/license/generate_offline_license.go 第 49 行
LicenseExpires: jwt.NewNumericDate(now.Add(time.Hour * 24 * 365 * 10)), // 10 years
```

**10 年 > 5 年**，导致验证失败！

### 解决方案

已修改代码支持 30 年 license：

1. **修改验证逻辑**：`enterprise/coderd/license/license.go` 第 706 行
   ```go
   yearsHardLimit := time.Now().Add(30 /* years */ * 365 * 24 * time.Hour)
   ```

2. **修改生成逻辑**：`enterprise/coderd/license/generate_offline_license.go`
   ```go
   ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour * 24 * 365 * 30)), // 30 years
   LicenseExpires: jwt.NewNumericDate(now.Add(time.Hour * 24 * 365 * 30)), // 30 years
   ```

### 生成新的 License

```bash
# 重新生成 license（有效期 5 年）
coder generate-license -o license.jwt

# 重启 Coder 服务
```

---

## 相关文档

- [Coder API 文档](https://coder.com/docs/api)
- [Coder OIDC 配置](https://coder.com/docs/admin/auth/oidc)
- [Keycloak 官方文档](https://www.keycloak.org/documentation)