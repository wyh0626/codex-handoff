# codex-handoff

在终端中多选本机 Codex 项目，把选中项目的会话统一导出成一个交接包；接手人导入时，再把旧项目路径绑定到自己的本地 Git 仓库目录。

只迁移 Codex 项目会话和脱敏后的 Git 仓库地址，不打包源码，不迁移 Codex 登录信息。

> 非 OpenAI 官方工具。本项目基于 MIT 许可的 [ahmojo/codex-claude-transfer](https://github.com/ahmojo/codex-claude-transfer) 开发。

## 快速安装

macOS / Linux：

```bash
curl -fsSL https://raw.githubusercontent.com/wyh0626/codex-handoff/main/install.sh | sh
```

安装脚本会根据系统和 CPU 下载最新 Release，校验 SHA-256，然后安装到 `~/.local/bin/codex-handoff`。Windows 用户可以从 [Releases](https://github.com/wyh0626/codex-handoff/releases/latest) 下载压缩包。

也可以从源码构建（Go 1.23+）：

```bash
git clone https://github.com/wyh0626/codex-handoff.git
cd codex-handoff
go build -o codex-handoff ./cmd/codex-handoff
```

## 使用

交接人直接运行：

```bash
codex-handoff
```

在终端中多选项目后，生成一个 `codex-project-handoff.codexbundle`。默认包含活跃和归档会话，并对疑似密钥执行脱敏。

接手人准备好代码仓库后运行：

```bash
codex-handoff import
```

导入界面会显示每个项目的 Git 仓库地址，并让接手人把旧路径分别绑定到自己的本地目录。

## 安全测试自己的交接包

仓库自带隔离测试脚本。它会创建临时 Codex Home，先 dry-run，再真实导入，结束后删除临时目录；不会使用真实的 `~/.codex`：

```bash
EXPECTED_SESSIONS=260 ./scripts/test-handoff-local.sh \
  ./codex-handoff ./codex-project-handoff.codexbundle
```

## 数据范围

会进入交接包：

- 选中项目的 Codex 活跃会话和归档会话；
- 会话标题、消息、工具记录、原始工作目录；
- 项目与会话的归属关系；
- Git 仓库地址，URL 中的账号、密码、Token 和查询参数会被清除；
- bundle 清单和 SHA-256 完整性信息。

不会进入交接包：

- 项目源码、`.git`、依赖和构建产物；
- Codex `auth.json`、Cookie、账号登录状态；
- 全局配置和未选择项目的会话。

会话正文仍可能包含业务信息。导出工具默认做规则脱敏，但分享前仍应检查，并使用公司认可的传输渠道；必要时启用 `age` 加密。

更完整的命令和安全边界见 [中文使用说明](README-HANDOFF.zh-CN.md)。

## 开源与上游

本仓库保留上游 MIT `LICENSE` 和提交历史，并在其 session 扫描、bundle 校验、脱敏、导入回滚和路径重映射能力上，增加了面向离职交接的多项目工作流。
