# Cloudflare Access

## 当前方案

- Cloudflare Zero Trust team: `<your-team-name>`
- Team domain: `https://<your-team-name>.cloudflareaccess.com`
- Public entry: `https://desk.example.com`
- Identity provider: `One-Time PIN`
- Local service: `http://127.0.0.1:47831`

## 架构

链路如下：

`browser -> Cloudflare Access -> Cloudflare Tunnel -> deskctrl -> PowerShell scripts`

职责拆分：

- `deskctrl` 只提供本地 HTTP API 和页面
- `cloudflared` 负责把本地服务接到 Cloudflare
- `Cloudflare Access` 负责认证、会话和访问策略
- 应用内不再保留 access key、session cookie 或登录页

## Zero Trust 配置

当前 Access 应用建议保持如下：

- Application type: `Self-hosted`
- Application name: `DeskCtrl`
- Domain: `desk.example.com`
- Target: `127.0.0.1:47831`
- Protocol: `HTTP`
- Session duration: `30 days`
- Identity provider: `One-Time PIN`

访问策略建议：

- Action: `Allow`
- Selector: `Emails`
- Value: 你的邮箱地址

## 验证

访问 `https://desk.example.com` 时：

1. 先进入 Cloudflare Access 登录页
2. 输入邮箱地址
3. 收到 One-Time PIN 邮件
4. 验证通过后进入 DeskCtrl 页面

## 运维说明

- 本地开发直接访问 `http://127.0.0.1:47831`
- 公网访问统一走 `https://desk.example.com`
- 当前推荐通过命令行参数或系统服务启动 `cloudflared`，不依赖仓库内配置文件
- 如果未来切换 GitHub / Google / 其他 IdP，只改 Cloudflare Access，不改应用代码
