# AGENTS.md

## 核心原则

- 不要向前兼容
- mobile first
- 优先选择最擅长当前问题的技术，不强制全用 Go
- 可以混合开发：Go 负责服务层、接口层、用户层；Python 或 PowerShell 可以负责更合适的系统能力调用
- 对于不需要长期驻留内存的本地系统操作，优先接受一次性脚本调用
- 在 Windows 场景下，如果 `pwsh.exe` 调用更直接、更稳，就优先使用 `ps1`

## 关于并行执行

- Default to serial shell execution.
- Only clearly read-only inspection commands may run in parallel.
- Never run `git` state-changing commands in parallel.
- Never run file-writing, process-management, package-manager, or migration commands in parallel.
- If unsure whether a command is read-only, run it serially.

## 关于 Wrangler

- `wrangler whoami` 会超时，但这不代表权限有问题。
- 你拥有权限，并且其他 `wrangler` 子命令都能成功。
