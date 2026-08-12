# Codex 项目会话交接工具

`codex-handoff` 是一个运行在终端里的 Codex 项目会话交接工具：交接人从本机全部有会话的 Codex 项目中多选，统一生成一个 `.codexbundle`；接手人导入时，将每个旧项目路径指向自己已经准备好的本地仓库目录。

它不打包代码仓库，也不迁移 Codex 账号。

## 能迁移什么

- 选中项目的 Codex 活跃会话和归档会话
- 每条会话的标题、消息、工具记录和原始工作目录 `cwd`
- 项目与会话的归属关系
- 每个已选项目的 Git 仓库地址（会清除 URL 凭据）
- bundle 清单与 SHA-256 完整性校验

不会迁移：

- 项目源码、`.git`、构建产物或依赖目录
- `auth.json`、Codex 登录状态、Cookie
- `config.toml`、全局 Memory、未选择项目的会话
- `.env`、私钥等本地文件本身，或外部系统登录状态（但它们若曾出现在会话文本或命令输出中，仍属于会话内容）

项目列表来自 Codex 会话中记录的 `cwd`。没有任何会话的空项目不会显示，因为它没有可交接的上下文。

## 构建

需要 Go 1.23 或更高版本：

```bash
go build -o codex-handoff ./cmd/codex-handoff
```

公开发布后，macOS / Linux 用户也可以直接安装最新的、经过 SHA-256 校验的 Release：

```bash
curl -fsSL https://raw.githubusercontent.com/wyh0626/codex-handoff/main/install.sh | sh
```

Windows 用户从 GitHub Releases 下载 `codex-handoff_windows_amd64.zip`。

## 隔离测试自己的交接包

下面的脚本只使用自动创建的临时 Codex Home，不会写入真实的 `~/.codex`。它会先 dry-run，再执行一次完整导入，并在结束后删除临时目录：

```bash
EXPECTED_SESSIONS=260 ./scripts/test-handoff-local.sh \
  ./codex-handoff ./codex-project-handoff.codexbundle
```

如果测试其他交接包，可以不设置 `EXPECTED_SESSIONS`；脚本仍会检查 dry-run 没有写文件，以及正式导入至少恢复了一条会话。

## 交接人导出

直接运行：

```bash
./codex-handoff
```

终端会列出所有拥有可导出会话的 Codex 项目：

```text
Select project folders to hand off

[x] /Users/alice/work/project-alpha  (86 sessions)
[ ] /Users/alice/work/project-beta   (17 sessions)
[x] /Users/alice/work/project-gamma  (34 sessions)

Space: toggle    Enter: confirm    /: filter
```

选择完成后，会生成单个文件：

```text
codex-project-handoff.codexbundle
```

专用交接入口默认包含归档会话，并对疑似 Token、API Key、私钥等内容执行脱敏。它不会读取或打包项目源码；只使用本地 Git 元数据记录项目的仓库地址，远程 URL 中的账号、密码、Token、查询参数会先被清除。

也可以使用非交互命令；`--project` 可以重复：

```bash
./codex-handoff export \
  --project /Users/alice/work/project-alpha \
  --project /Users/alice/work/project-gamma \
  --output my-handoff.codexbundle
```

`codex-handoff` 的非交互导出同样默认追加 `--redact` 和 `--include-archived`。如需保留原始内容，必须显式使用底层 `cct export --allow-secrets`，并自行承担敏感信息泄露风险。

如果某个旧项目目录已经删除，工具无法从本地 Git 自动取得远程地址，可以显式补充：

```bash
./codex-handoff export \
  --project /old/path/project-delta \
  --project-git /old/path/project-delta=git@git.example.com:team/project-delta.git
```

`--project-git` 可以重复，并且只能对应本次选择的项目。URL 中的凭据仍会在写入前清除。

## 接手人导入

接手人先自行 Clone 或准备项目代码，然后执行：

```bash
./codex-handoff import
```

选择交接包后，工具会先按项目展示“旧目录 → Git 仓库地址”。对于接手人电脑上不存在的每个目录，逐一选择：

- 指向接手人已有的本地项目目录；
- 暂时跳过；
- 创建与原路径相同的目录。

推荐选择“Point these sessions to a different folder”，把每个旧目录分别映射到自己的仓库目录。工具先 dry-run，展示新增、已存在、路径重映射和冲突数量；确认后才写入会话文件。`codex-handoff import` 默认同时恢复包内归档任务，仍保留在 Codex 的 `archived_sessions` 中。

例如导入提示会包含：

```text
/Users/alice/work/project-beta
  Git: git@git.example.com:team/project-beta.git
```

接手人用自己的 Git 凭据 Clone 后，再把该旧目录绑定到自己的本地 Clone 目录即可。没有配置远程地址、源目录已经不存在或不是 Git 仓库的项目，会明确显示“Git not recorded”，需要交接人另行提供地址。

导入不会直接改写 Codex SQLite 或 `session_index.jsonl`。可以选择让 Codex app-server 立即发现并核验新会话；如果发现失败，会保留已导入的 rollout 文件并提示重启或 resume。

## 查看交接包

导入前可以只读检查：

```bash
./codex-handoff inspect
```

## 安全边界

- bundle 是 ZIP 容器，但导入会先校验清单和 SHA-256。
- 导入拒绝路径穿越、绝对 bundle 路径和未声明文件。
- 分享前默认脱敏；如果会话含业务数据，仍建议通过公司认可的私有渠道传输。
- 可在导出向导中选择使用 `age` 公钥或口令加密。
- 接手人应使用自己的 Git、Codex、云平台和内部系统凭证。

## 与上游项目的关系

本实现基于 MIT 许可的 [ahmojo/codex-claude-transfer](https://github.com/ahmojo/codex-claude-transfer) v1.9.0（基线提交 `1515d8b`）。复用了其 session 扫描、bundle、Secret 扫描、校验、导入回滚、cwd 重映射及 Codex reconciliation 能力，并新增：

- `codex-handoff` 专用命令；
- 终端多项目选择；
- 重复 `--project` 的多项目单包导出；
- 多项目包的项目清单字段；
- 逐项目记录脱敏后的 Git 仓库地址，并在导入绑定时展示；
- 面向交接场景的默认归档会话与默认脱敏策略；
- 多项目导入时逐项目绑定本地目录，禁止把所有项目快捷映射到同一目录。

仓库保留了上游的 `LICENSE`、提交历史和完整测试套件。
