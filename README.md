# DeskCtrl

## Summary

DeskCtrl 是一个本地 Web 控制面板，用来查看当前桌面截图、读取当前系统音量，并控制系统主音量。

首版目标是先把本地能力跑通，不做向前兼容。应用自身默认只监听本机地址，公网认证交给 Cloudflare Access。

## V1 Goals

- 在浏览器中查看“当前桌面截图”
- 在浏览器中查看“当前系统音量”
- 在浏览器中直接设置系统音量
- 在浏览器中输入提示词并选择执行 `pi -p` 或 `codex`
- 在浏览器中浏览用户家目录下的文件列表
- 提供尽量简单直接的本地接口，方便后续接入通道层

## Product Shape

- 一个本地后端服务
- 一个简单的 Web 页面
- 页面打开后可以主动刷新截图
- 页面可以轮询或手动刷新当前音量
- 页面提供滑块或按钮调整系统音量
- 页面提供提示词输入区并展示 `pi` 或 `codex` 返回结果
- 页面提供只读文件浏览区，支持进入目录和返回上级

## Suggested Architecture

### Backend

- 提供 HTTP API
- 截图接口：返回最新截图图片
- 音量读取接口：返回当前系统主音量数值
- 音量设置接口：接收目标音量并立即生效
- Pi 接口：接收提示词和运行方式，返回 `pi -p` 或 `codex` 的文本结果
- 文件浏览接口：返回用户家目录范围内的目录列表

### Frontend

- 一个单页面
- 展示截图预览区
- 展示当前音量数值
- 提供音量滑块
- 提供截图刷新按钮

### Access Layer

- `deskctrl` 只监听 `127.0.0.1:47831`
- `cloudflared` 负责公网连通
- `desk.example.com` 由 Cloudflare Access 保护
- 当前身份方式为邮箱 `One-Time PIN`

## API Draft

- `GET /api/screenshot`
  - 返回当前桌面截图
- `GET /api/volume`
  - 返回当前系统音量，例如 `{"level": 42}`
- `POST /api/volume`
  - 请求体例如 `{"level": 60}`
  - 设置系统音量并返回最新值
- `POST /api/pi`
  - 请求体例如 `{"runner": "pi", "prompt": "总结当前目录用途"}`
  - `runner` 支持 `pi` 和 `codex`，缺省为 `pi`
  - 返回例如 `{"result": "..."}` 的文本结果
- `GET /api/files`
  - 查询参数 `path` 可选，缺省为当前用户家目录
  - 只允许浏览用户家目录范围内的目录
  - 返回当前目录、父目录和目录项列表

## Implementation Notes

- 截图能力可以参考现有截图技能的实现思路
- 音量能力需要调用 Windows 本地音频接口
- 首版本地接口不做应用内鉴权
- 首版优先直接可跑，UI 不追求复杂
- 如果接入 Cloudflare 通道，优先把认证放在 Cloudflare Access，而不是应用内重复做一套登录

## Auth

- 本地访问：`http://127.0.0.1:47831`
- 公网访问：`https://desk.example.com`
- 应用内无登录逻辑、无 access key、无本地 session
- Cloudflare Access 负责认证和会话管理
- 具体配置见 [ACCESS_SETUP.md](./ACCESS_SETUP.md)

## Out of Scope For Now

- 应用内用户登录系统
- 多显示器高级控制
- 应用级音量控制
- 历史截图归档
- 文件写入、删除、重命名或下载

## Acceptance Criteria

- 本机打开页面后能看到当前桌面截图
- 页面能显示当前系统主音量
- 调整页面控件后，系统音量会同步变化
- 输入提示词后，页面能显示所选运行方式返回的文本
- 文件浏览区能刷新家目录列表，并进入可见子目录
- 刷新页面后，状态仍可重新读取
