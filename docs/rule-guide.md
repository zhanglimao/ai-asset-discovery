# 规则编写与使用指南

本文档详细介绍如何为 AI Asset Discovery 编写检测规则，帮助开发者贡献新 Agent 的检测能力。

---

## 目录

1. [规则文件结构](#规则文件结构)
2. [AgentRule 完整字段说明](#agentrule-完整字段说明)
3. [简化检测指纹（Features）⭐](#简化检测指纹features-推荐)
4. [简化路径规则（Paths）⭐](#简化路径规则paths-推荐)
5. [命令探测规则（Probe）⭐](#命令探测规则probe-推荐)
6. [进程检测规则（Process）](#进程检测规则process)
7. [文件系统检测规则（Files）](#文件系统检测规则files)
8. [IDE 扩展检测规则（IDE）](#ide-扩展检测规则ide)
9. [配置提取规则（Config）](#配置提取规则config)
10. [技能发现规则（Skills）](#技能发现规则skills)
11. [包管理器检测规则（Package）](#包管理器检测规则package)
12. [二进制 PATH 检测规则（Binary）](#二进制-path-检测规则binary)
13. [置信度体系](#置信度体系)
14. [匹配逻辑详解](#匹配逻辑详解)
15. [路径规范](#路径规范)
16. [实战案例](#实战案例)
17. [常见陷阱](#常见陷阱)

---

## 规则文件结构

规则以 YAML 文件存储，顶层结构如下：

```yaml
version: "1.0"

agents:
  - name: agent-1
    # ... 规则定义
  - name: agent-2
    # ... 规则定义
```

- **`version`**：规则格式版本号，当前为 `"1.0"`
- **`agents`**：规则数组，每项定义一个 Agent

支持从目录加载多个 `.yaml` / `.yml` 文件，引擎会自动合并所有规则。

---

## AgentRule 完整字段说明

规则支持**两种语法**：推荐使用简化语法（`features` + `paths` + `probe`），引擎会自动转换为内部详细字段。高级场景可使用详细语法（`process`/`files`/`ide` 等）。

### 推荐：简化语法（`features` / `paths` / `probe`）

| 字段 | 类型 | 必需 | 说明 |
|------|------|------|------|
| `name` | string | ✅ | 唯一标识符，小写英文、短横线连接（如 `claude-code`） |
| `display_name` | string | ✅ | 显示名称（如 `"Claude Code"`） |
| `description` | string | 否 | 描述文本 |
| `category` | string | ✅ | 分类，见下方分类表 |
| `min_confidence` | string | 否 | 最低置信度，默认 `possible` |
| `features` | FeaturesRule | 否 | **推荐**：简化检测指纹，引擎自动转换为 process/package/binary/ide 规则 |
| `paths` | PathRule[] | 否 | **推荐**：简化路径列表，引擎自动转换为 files 规则 |
| `probe` | ProbeRule | 否 | **推荐**：命令探测，执行命令确认 Agent 类型并提取版本 |
| `skills` | SkillRule | 否 | 技能发现规则（需设置 `enabled: true`） |
| `config` | ConfigRule | 否 | 配置提取规则 |

### 高级：详细语法（`process` / `files` / `ide` / `package` / `binary`）

| 字段 | 类型 | 必需 | 说明 |
|------|------|------|------|
| `process` | ProcessRule | 否 | 进程检测规则（详细控制 match_logic / weight） |
| `files` | FileRule[] | 否 | 文件系统检测规则（详细控制 file_type / contains / max_depth） |
| `ide` | IDERule | 否 | IDE 扩展检测规则（详细控制 paths / keywords / depends） |
| `package` | PackageRule | 否 | 包管理器检测规则（详细控制 managers） |
| `binary` | BinaryRule | 否 | 二进制 PATH 检测规则（详细控制 pattern type / weight） |

> **注意**：简化语法和详细语法可以混用。如果同时配置了 `features` 和 `process`，`features.processes` 会**追加**到 `process.name_patterns` / `process.cmd_patterns` 中。

### 分类（category）枚举

| 值 | 含义 | 典型场景 |
|----|------|----------|
| `cli_agent` | 命令行 AI Agent | Claude Code, Gemini CLI, Aider, Codex CLI |
| `ide_extension` | IDE / 编辑器扩展 | GitHub Copilot, Cline, Continue |
| `desktop_app` | 桌面 AI 应用 | ChatGPT Desktop, Windsurf |
| `desktop_assistant` | 桌面 AI 助手 | WorkBuddy, Kimi, 豆包 |
| `browser_agent` | 浏览器 AI Agent | Manus, Fellou, Genspark |
| `agent_framework` | Agent 开发框架 | LangChain, AutoGen, CrewAI, Dify, Coze |
| `sdk_detection` | LLM SDK 依赖检测 | 通用 LLM SDK 痕迹 |
| `web_tool` | Web 端 AI 工具 | Bolt.new, Replit Agent |

---

## 简化检测指纹（Features）⭐ 推荐

`features` 是最简单的规则编写方式。只需列出进程名、包名、二进制名、扩展 ID 等字符串，引擎自动处理匹配逻辑、权重和模式类型。

### 数据结构

```yaml
features:
  processes:                  # 进程名或命令行子串（区分大小写 contains 匹配）
    - claude-code
    - "claude code"
  packages:                   # 包管理器中的包名（exact 匹配）
    - "@anthropic-ai/claude-code"
    - aider-chat
  binaries:                   # $PATH 中的二进制名（exact 匹配）
    - claude
    - aider
  extensions:                 # IDE 扩展 ID（exact + glob 匹配）
    - "GitHub.copilot"
    - "GitHub.copilot-chat"
  agent_signals:              # Agent 能力信号关键词
    - "createAgent"
    - "tool_use"
  version_regex: "([0-9]+\\.[0-9]+\\.[0-9]+)"  # 版本号提取正则
  version_flag: "--version"                     # 二进制版本标志（默认 --version）
```

### 转换规则

引擎在加载时将 `features` 自动转换为对应的详细字段：

| features 字段 | 转换为 | 行为 |
|---|---|---|
| `processes` | `process.name_patterns` + `cmd_patterns` | 每个值同时添加到 name 和 cmd，type=word, weight=5/8 |
| `packages` | `package.packages` | type=exact, managers 默认 `[pip, pip3, npm, apt, brew]` |
| `binaries` | `binary.names` | type=exact, version_flag/version_regex 同步转发 |
| `extensions` | `ide.ext_ids` | 追加到 ext_ids 列表（需配合 `ide.scan_paths` 指定扫描目录） |
| `agent_signals` | `ide.agent_signals` | 追加到 agent_signals 列表 |
| `version_regex` | `process.version_regex` + `binary.version_regex` | 同时设置到 process 和 binary |

### 使用建议

- 绝大多数 Agent 只需 `features` + `paths` + `probe` 三条简化字段即可覆盖
- 需要精细控制权重或 `match_logic` 时，使用详细语法 `process` / `binary`

---

## 简化路径规则（Paths）⭐ 推荐

```yaml
paths:
  - path: ~/.claude-code          # 路径（支持变量展开）
  - path: ~/.claude               # 兼容旧版
  - path: "%APPDATA%/Claude"      # Windows 路径
    os: windows
```

| 字段 | 类型 | 必需 | 说明 |
|------|------|------|------|
| `path` | string | ✅ | 文件/目录路径，支持 `~`、`%VAR%` |
| `required` | bool | 否 | 默认 `false`。`true` 时该路径必须存在 |
| `os` | string | 否 | 限定操作系统：`linux` / `darwin` / `windows`（默认 all） |

引擎自动将 `paths` 转换为 `files` 规则（`file_type: directory`）。

---

## 命令探测规则（Probe）⭐ 推荐

通过执行命令来确认 Agent 类型并提取版本号。

```yaml
probe:
  command: claude                 # 命令（必须在 $PATH 中）
  args: ["--version"]             # 参数
  version_regex: "([0-9]+\\.[0-9]+\\.[0-9]+)"  # 版本提取正则
  expected_output: "Claude Code"  # 可选：输出中必须出现的子串
```

| 字段 | 类型 | 必需 | 说明 |
|------|------|------|------|
| `command` | string | ✅ | 要执行的命令 |
| `args` | string[] | 否 | 命令参数（如 `["--version"]`） |
| `version_regex` | string | 否 | 从输出中提取版本的正则（第一捕获组） |
| `expected_output` | string | 否 | 输出中必须包含的子串（大小写不敏感），空表示执行成功即匹配 |

---

## 进程检测规则（Process）

进程检测通过扫描 `/proc` 文件系统，将运行中的进程与规则进行匹配。

### 数据结构

```yaml
process:
  match_logic: or       # or（默认）| and
  name_patterns:        # 匹配 /proc/PID/comm（进程名，最多15字符）
    - type: contains
      value: "claude-code"
      weight: 10
  cmd_patterns:         # 匹配 /proc/PID/cmdline（完整命令行）
    - type: regex
      value: "claude.*agent"
      weight: 8
  exe_patterns:         # 匹配 /proc/PID/exe（可执行文件路径）
    - type: contains
      value: "/claude.exe"
      weight: 15
  dir_patterns:         # 匹配 /proc/PID/cwd（当前工作目录）
    - type: contains
      value: "/project"
      weight: 3
  version_regex: "claude[- ]v?([0-9]+\\.[0-9]+\\.[0-9]+)"
```

### PatternRule — 模式匹配规则

| 字段 | 类型 | 必需 | 说明 |
|------|------|------|------|
| `type` | string | ✅ | 匹配类型：`exact` / `contains` / `regex` |
| `value` | string | ✅ | 匹配值 |
| `weight` | int | 否 | 权重（越高越可信），用于置信度提升 |

### 三种匹配类型（注：`word` 为 `normalizeFeatures` 自动生成的内部类型）

```yaml
# exact：精确相等（区分大小写）
- type: exact
  value: "claude-code"

# contains：子串包含（区分大小写）
- type: contains
  value: "Claude"

# regex：正则表达式（.NET 兼容正则，支持反向引用、零宽断言等高级特性）
- type: regex
  value: "^claude(-\w+)?$"

# word：整词边界匹配（内部使用，由 features.processes 自动生成）
# 类似于 contains 但要求匹配值在字符串中作为独立单词出现，
# 边界为字符串起止、非字母数字字符或大小写转换（CamelCase）。
- type: word
  value: "omp"
```

### 四种匹配类型对比

| 类型 | 匹配方式 | 示例 |
|------|----------|------|
| `exact` | 字符串精确相等（忽略大小写） | `"omp"` 只匹配 `"omp"` |
| `contains` | 子串包含（忽略大小写） | `"omp"` 匹配 `"WUDFCompanionHost"` |
| `word` | 整词边界包含（忽略大小写） | `"omp"` 匹配 `"/usr/bin/omp"` 但不匹配 `"Companion"` |
| `regex` | .NET 兼容正则 | `"^claude(-\w+)?$"` 匹配 `"claude-code"` |

> **注意**：`word` 类型是 `normalizeFeatures` 自动生成的内部类型，用于避免 `contains` 匹配中的误报。普通规则编写请使用 `exact` / `contains` / `regex`。

### match_logic 详解

#### `or`（默认）
任一配置的字段命中即视为匹配。适用于宽松匹配场景。

```yaml
process:
  match_logic: or
  name_patterns:
    - type: contains
      value: "aider"
  cmd_patterns:
    - type: contains
      value: "aider"
```
→ 进程名包含 "aider" **或** 命令行包含 "aider" 即命中。

#### `and`
**所有已配置的字段必须全部命中**。未配置的字段自动视为通过。

```yaml
process:
  match_logic: and
  name_patterns:
    - type: regex
      value: "^(python3?|node)$"
  cmd_patterns:
    - type: regex
      value: "(langchain|llama_index|autogen|crewai)"
```
→ 进程名必须是 python3/python/node **且** 命令行包含框架名才命中。用于排除 bash 包装器和误报。

### 关键实现细节

1. **进程名截断**：`/proc/PID/comm` 最多 15 字符（macOS `ps` 的 `comm` 字段截断 20 字符），名称可能被截断（如 `python3.11` → `python3`），使用 `regex` 或 `contains` 而非 `exact`
2. **cmdline 空格**：`/proc/PID/cmdline` 用 `\x00` 分隔参数，引擎自动转为空格
3. **自过滤**：引擎自动排除自身进程（discovery）及其父 Shell
4. **版本提取**：`version_regex` 使用捕获组 `()` 提取版本号，优先匹配 cmdline，其次匹配 exe 路径
5. **置信度提升**：≥2 个字段命中时，`ghost` → `possible`，其他 → `confirmed`
6. **weight 字段**：`PatternRule.Weight` 用于标记各模式的主观重要程度（文档参考），但当前的匹配算法按命中字段数量（≥2）而非权重高低来提升置信度

---

## 文件系统检测规则（Files）

检测磁盘上的文件/目录证据。

### 数据结构

```yaml
files:
  - path: ~/.claude-code          # 路径（支持变量展开）
    file_type: directory           # file | directory
    required: false                # 是否必需（true 时缺少则跳过整条规则）
    contains: "apiKey"             # 可选：文件内容必须包含此字符串
    max_depth: 2                   # 可选：目录扫描最大深度
    os: linux                      # 可选：OS 过滤（linux/darwin/windows/all）
```

### 字段说明

| 字段 | 类型 | 必需 | 说明 |
|------|------|------|------|
| `path` | string | ✅ | 文件/目录路径，支持 `~`、`%VAR%` |
| `file_type` | string | ✅ | `file`（普通文件）或 `directory`（目录） |
| `required` | bool | 否 | 默认 `false`。`true` 时该路径必须存在，否则整条规则不产生结果 |
| `contains` | string | 否 | 检查文件内容（仅前 10KB），匹配子串 |
| `max_depth` | int | 否 | 目录遍历深度限制 |
| `os` | string | 否 | 限定操作系统：`linux` / `darwin` / `windows` / `all` |

### 必需/可选逻辑

```yaml
files:
  - path: ~/.aider.conf.yml     # 可选：存在时增加证据
    file_type: file
    required: false
  - path: ~/.aider              # 必需：不存在则放弃此规则
    file_type: directory
    required: true
```

- 如果存在任意 `required: true` 的文件规则，必须至少命中一个 `required` 路径，否则整条规则不匹配
- 如果全部为 `required: false`，命中任意一个即可

---

## IDE 扩展检测规则（IDE）

检测 VS Code / Cursor / Windsurf 等 IDE 中安装的 AI 扩展。

### 数据结构

```yaml
ide:
  paths:                                    # 扫描的扩展目录
    - ~/.vscode/extensions
    - ~/.cursor/extensions
    - ~/.vscode-server/extensions
  ext_ids:                                  # 精确扩展 ID
    - "GitHub.copilot"
    - "GitHub.copilot-chat"
  keywords:                                 # 关键词匹配
    - copilot
    - agent
  depends:                                  # 依赖包名
    - "@anthropic-ai/sdk"
  agent_signals:                            # Agent 能力信号
    - "createAgent"
    - "registerAgent"
  config_keys:                              # 配置项提取
    - field: "enable"
      key_path: "enable"
```

### 匹配优先级

IDE 扫描器采用**分层匹配**：

1. **`ext_ids` 精确匹配**（优先级最高）：如果配置了 `ext_ids`，则**仅按扩展 ID 精确匹配**，不会 fallback 到关键词匹配。这可以防止跨规则污染（如 `github-copilot` 规则的关键词 `"ai"` 误匹配到 Continue 或其他扩展）

2. **关键词/依赖启发式匹配**（仅在无 `ext_ids` 时）：检查扩展的 `package.json` 中 categories、keywords、displayName、dependencies

3. **Agent 信号检测**：扫描扩展的 `package.json` 中 `contributes`、`activationEvents` 等字段，匹配 `agent_signals` 中列出的字符串。命中后置信度提升至 `confirmed`

### 扩展 ID 说明

扩展 ID 格式为 `publisher.extension`，从扩展目录下的 `package.json` 中读取 `publisher` 和 `name` 字段拼接而成（如 `GitHub.copilot`），并非从目录名提取：
```
~/.vscode/extensions/github.copilot-1.2.3/
                        ├── package.json    → publisher: "GitHub", name: "copilot"
                        └── ...
                                       → 扩展ID = "GitHub.copilot"
```

---

## 配置提取规则（Config）

从 Agent 的配置文件中提取关键信息（模型、API Key、版本等）。

### 数据结构

```yaml
config:
  format: env               # json | yaml | yml | env | toml
  paths:
    - ~/.aider.conf.yml
    - .env
  field_map:                # target_field -> source_path_in_config
    model: "AIDER_MODEL"
    api_key: "OPENAI_API_KEY"
    editor: "AIDER_EDITOR"
```

### 支持的配置格式

| format | 说明 | 解析方式 |
|--------|------|----------|
| `json` | JSON 配置文件 | `json.Unmarshal` |
| `yaml` / `yml` | YAML 配置文件 | `yaml.Unmarshal` |
| `env` | 环境变量文件 | 按 `KEY=VALUE` 行解析 |
| `toml` | TOML 配置文件 | 简易 TOML 解析器（支持 `[section]`） |

### field_map 嵌套取值

使用 `.` 分隔路径访问嵌套字段：

```yaml
config:
  format: json
  paths:
    - ~/.config/agent/settings.json
  field_map:
    model: "llm.model"             # 访问 {"llm": {"model": "..."}}
    temperature: "llm.temperature" # 访问 {"llm": {"temperature": 0.7}}
```

---

## 技能发现规则（Skills）

从 Agent 的技能目录中自动发现和解析技能文件。

### 数据结构

```yaml
skills:
  enabled: true                         # ⭐ 必须设为 true 才会激活技能发现（默认 false）
  scan_paths:                           # 扫描目录
    - ~/.cline/skills
    - ~/.claude-code/skills
  max_depth: 3                          # 扫描深度（默认 3）
  max_size_kb: 100                      # 文件大小上限 KB（默认 100）
  auto_discover: false                  # 设为 false 可关闭自动探测（默认 true）
```

> **注意**：技能发现仅识别文件名严格为 `SKILL.md`（大小写不敏感）的文件，按 [Agent Skills 规范](https://agentskills.io/specification) 解析 YAML frontmatter。不支持通过扩展名过滤其他格式。

### auto_discover 自动探测

当 `auto_discover: true`（**默认**）时，引擎会自动在 Agent 的文件证据目录下探测以下子目录名：

```yaml
# 自动探测的目录名：
skills, agents, tools, instructions, prompts, rules, commands, workflows, .skills, .agent
```

示例：规则中 `files` 包含 `~/.cline`，且 `auto_discover: true`，引擎会自动扫描：
- `~/.cline/skills/`、`~/.cline/agents/`、`~/.cline/tools/` 等

无需在 `scan_paths` 中逐一列举，大幅简化规则维护。auto_discover 不会重复扫描已在 `scan_paths` 中显式配置的路径。

### 技能文件格式

技能发现按 [Agent Skills 规范](https://agentskills.io/specification) 工作：

- **文件名**：仅识别 `SKILL.md`（大小写不敏感）
- **格式**：Markdown + YAML frontmatter（`---` 包裹的 YAML）
- **回退**：如无 frontmatter，从 Markdown 标题段落提取 name/description/tools/prompt

### Markdown 技能文件格式

推荐使用 YAML frontmatter：

```markdown
---
name: code-review
description: Review code changes for best practices
tools:
  - Bash
  - Read
  - Edit
parameters:
  depth: full
  language: python
---

# code-review

Skill for reviewing code changes...
```

Frontmatter 中 `---` 包裹的 YAML 会被自动解析并映射到 Skill 结构体。

### 默认值

| 字段 | 默认值 |
|------|--------|
| `enabled` | `false` |
| `max_depth` | `3` |
| `max_size_kb` | `100` |
| `auto_discover` | `true` |

> **注意**：`enabled` 默认为 `false`，必须显式设为 `true` 才会激活技能扫描。技能扫描会读取文件内容（最大 `max_size_kb` KB）。

---

## 包管理器检测规则（Package）

通过系统包管理器（npm、pip、apt、brew 等）检测已安装的 AI 软件包。

### 数据结构

```yaml
package:
  managers:                     # 包管理器列表
    - npm
    - pip
  packages:                     # 包名匹配模式
    - name: "@anthropic-ai/claude-code"
      type: exact               # exact | regex
    - name: "aider-chat"
      type: exact
  version_regex: "([0-9]+\\.[0-9]+\\.[0-9]+)"  # 可选：从包版本字符串中提取版本号
```

> **`version_regex`**：当包管理器返回的版本字符串格式不标准时（如 `"v1.2.3"`），可用此正则的捕获组提取干净版本号。

### 支持的包管理器

| manager | 命令 | 输出格式 |
|---------|------|----------|
| `npm` | `npm list -g --depth=0` | `├── package@version` |
| `pip` / `pip3` | `pip list --format=json` | JSON 数组 |
| `apt` | `apt list --installed` | `package/stable,now version arch` |
| `brew` | `brew list --versions` | `package version1 version2` |
| `cargo` | `cargo install --list` | `package v1.2.3:` |
| `gem` | `gem list --local` | `package (1.2.3, 1.1.0)` |

### 匹配模式

```yaml
packages:
  - name: "@anthropic-ai/claude-code"   # exact：精确匹配包名
    type: exact
  - name: "^(openai|anthropic)$"         # regex：正则匹配
    type: regex
```

---

## 二进制 PATH 检测规则（Binary）

通过 `$PATH` 查找 CLI 二进制程序，调用 `--version` 提取版本号。

### 数据结构

```yaml
binary:
  names:                                # 二进制名称模式
    - type: exact
      value: "claude"                   # which claude
    - type: contains
      value: "aider"                    # 部分匹配
    - type: regex
      value: "^gemini(-cli)?$"          # 正则匹配（遍历 PATH 目录）
  version_flag: "--version"             # 版本查询标志
  version_regex: "([0-9]+\\.[0-9]+\\.[0-9]+)"  # 版本号提取正则
```

> **`regex` 模式**：当 `type: regex` 时，扫描器会遍历 `$PATH` 中所有目录，用正则匹配文件名（而非调用 `exec.LookPath`）。适用于二进制名不确定的场景（如 `python3.10` vs `python3.11`）。

### 工作流程

1. 对 `exact` / `contains` 类型调用 `exec.LookPath(value)` 查找二进制
2. 对 `regex` 类型，遍历 `$PATH` 所有目录进行文件名匹配
3. 找到后执行 `<binary> <version_flag>` 获取版本输出
4. 用 `version_regex` 捕获组提取版本号

---

## 置信度体系

| 级别 | JSON 值 | 含义 | 触发条件 |
|------|---------|------|----------|
| **Confirmed** | `"confirmed"` | 确认运行 | 多维度交叉验证命中、Agent 模式信号明确 |
| **Possible** | `"possible"` | 可能运行 | 单一维度命中（进程/文件/扩展任一匹配） |
| **Ghost** | `"ghost"` | 历史痕迹 | 仅残留文件或 SDK 依赖，Agent 未实际运行 |

### 置信度提升规则

1. **进程检测**：单个字段命中 → `min_confidence`；≥2 个字段命中时：原置信度为 `ghost` → 提升至 `possible`；原置信度为其他值 → 提升至 `confirmed`
2. **IDE 扩展检测**：命中 `agent_signals` → 自动提升至 `confirmed`
3. **综合**：最终按各维度中最高置信度取值（跨类型合并时保留最高置信度）

---

## 匹配逻辑详解

### 进程检测：`or` vs `and`

```yaml
# 场景1：JS Agent —— node 进程很常见，必须配合 cmd 匹配
process:
  match_logic: and
  name_patterns:
    - type: regex
      value: "^(node|python3?)$"
  cmd_patterns:
    - type: contains
      value: "@anthropic-ai/claude-code"

# 场景2：ELF 原生 Agent —— 进程名唯一
process:
  match_logic: or
  name_patterns:
    - type: contains
      value: "claude-code"
```

**选择建议**：
- 进程名唯一（如 `aider`、`claude-code` 原生二进制）→ `or`
- 进程名是通用运行时（如 `node`、`python`）→ `and`
- 桌面应用进程名普遍（如 `ChatGPT`、`Electron`）→ `or` + 多个 pattern 提升置信度

### IDE 检测：exact ID vs keyword

**首选 `ext_ids`**，仅在无法确定扩展 ID 时使用 `keywords`：

```yaml
# ✅ 推荐：精确 ID 匹配
ide:
  ext_ids:
    - "GitHub.copilot"

# ⚠️ 谨慎：关键词匹配（可能产生交叉误报）
ide:
  keywords:
    - "copilot"
    - "AI assistant"
```

**重要**：`ext_ids` 和 `keywords` 是互斥的匹配路径。如果同时配置了 `ext_ids`，则**仅通过 `ext_ids` 匹配**，完全不检查 `keywords`（避免跨规则污染）。仅在没有 `ext_ids` 时才使用 `keywords`。

---

## 路径规范

### 路径变量展开

| 变量 | Linux | macOS | Windows |
|------|-------|-------|---------|
| `~/` | `/home/user/` | `/Users/user/` | `C:\Users\user\` |
| `%APPDATA%` | — | — | `C:\Users\user\AppData\Roaming\` |
| `%LOCALAPPDATA%` | — | — | `C:\Users\user\AppData\Local\` |
| `%USERPROFILE%` | — | — | `C:\Users\user\` |
| `%HOME%` | `/home/user/` | `/Users/user/` | `C:\Users\user\` |

### 典型路径模式

```yaml
# Linux/macOS home 目录
- path: ~/.config/claude-code

# VS Code 扩展（跨平台）
- path: ~/.vscode/extensions

# Windows 专用
- path: "%APPDATA%\\Code\\User\\settings.json"
  os: windows

# macOS 专用
- path: "~/Library/Application Support/Code/User/settings.json"
  os: darwin
```

---

## 实战案例

### 案例 1：npm 全局安装的 CLI Agent（Claude Code）

```yaml
- name: claude-code
  display_name: "Claude Code"
  description: "Anthropic Claude Code CLI agent"
  category: "cli_agent"
  min_confidence: possible
  paths:
    - path: ~/.claude
    - path: ~/.claude-code
  features:
    processes:
      - claude-code
    binaries:
      - claude
    packages:
      - "@anthropic-ai/claude-code"
    version_regex: "([0-9]+\\.[0-9]+\\.[0-9]+)"
  probe:
    command: claude
    args: ["--version"]
    version_regex: "([0-9]+\\.[0-9]+\\.[0-9]+)"
  skills:
    enabled: true
    scan_paths:
      - ~/.claude/skills
      - ~/.claude-code/skills
    auto_discover: true
```

**要点**：
- 用 `features` 简化语法，引擎自动处理进程/包管理器/二进制检测
- `paths` 兼容 `~/.claude`（旧版）和 `~/.claude-code`（新版）两个数据目录
- `probe` 通过执行 `claude --version` 确认类型并提取版本号
- `skills.auto_discover` 自动探测 paths 目录下的 skills/agents/tools 子目录

### 案例 2：pip 安装的 Python Agent 框架（LangChain）

```yaml
- name: langchain
  display_name: "LangChain/LangGraph"
  description: "Multi-agent orchestration framework"
  category: "agent_framework"
  min_confidence: ghost          # 仅 SDK 依赖，不一定是 Agent
  features:
    packages:
      - langchain
      - langgraph
```

**要点**：
- `min_confidence: ghost`：仅为 Python 包依赖，不证明 Agent 在运行
- `features.packages` 引擎默认检查 pip/npm/apt/brew 四个包管理器
- 如需精确限定包管理器，可用详细语法 `package.managers: [pip]`

### 案例 3：IDE 扩展（GitHub Copilot）

```yaml
- name: github-copilot
  display_name: "GitHub Copilot"
  description: "GitHub Copilot AI coding assistant"
  category: "ide_extension"
  min_confidence: possible
  features:
    extensions:
      - "GitHub.copilot"

- name: github-copilot-agent
  display_name: "GitHub Copilot Agent"
  description: "GitHub Copilot with agent mode enabled"
  category: "ide_extension"
  min_confidence: possible       # 扩展存在即 possible，命中 agent_signals 提升为 confirmed
  features:
    extensions:
      - "GitHub.copilot-chat"
    agent_signals:
      - "createAgent"
      - "registerAgent"
      - "tool_use"
```

**要点**：
- 两条规则区分 Copilot 普通模式和 Agent 模式
- `features.extensions` 自动匹配 ~/.cursor/extensions 下的扩展
- Agent 模式通过 `agent_signals` 确认，命中后置信度自动提升至 `confirmed`
- IDE 路径由引擎自动发现（vscode/cursor），无需手动配置

### 案例 4：桌面应用（ChatGPT Desktop）

```yaml
- name: chatgpt-desktop
  display_name: "ChatGPT Desktop"
  description: "OpenAI ChatGPT desktop application"
  category: "desktop_app"
  min_confidence: possible
  paths:
    - path: ~/Library/Application Support/ChatGPT
      os: darwin
    - path: ~/.config/ChatGPT
    - path: "%APPDATA%/ChatGPT"
      os: windows
  features:
    processes:
      - chatgpt
      - ChatGPT
```

**要点**：
- `paths` 使用 `os` 字段分别指定各平台路径
- `features.processes` 同时匹配进程名和命令行，区分大小写
- 引擎自动过滤自身进程和 Shell 包装器，无需 `exe_patterns` 排除逻辑

---

## 常见陷阱

### 1. 进程名截断
`/proc/PID/comm` 限制 15 字符。`python3.11` → `python3`。避免使用 `exact` 匹配进程名，用 `regex` 或 `contains`。
```yaml
# ❌ 错误
name_patterns:
  - type: exact
    value: "python3.11"

# ✅ 正确
name_patterns:
  - type: regex
    value: "^(python3?|python3\\.\\d+)$"
```

### 2. CLI Agent 的进程名是 node
通过 `npm install -g` 安装的 JS Agent，`/proc/PID/comm` 显示为 `node`（非 Agent 名）。必须使用 `match_logic: and` + `cmd_patterns`。
```yaml
# ❌ 错误：永远不会匹配
process:
  match_logic: or
  name_patterns:
    - type: contains
      value: "gemini-cli"    # comm 是 "node"，不包含 "gemini-cli"

# ✅ 正确
process:
  match_logic: and
  name_patterns:
    - type: regex
      value: "^(node|python3?)$"
  cmd_patterns:
    - type: contains
      value: "gemini-cli"
```

### 3. IDE 规则交叉污染
不配置 `ext_ids` 时，关键词匹配可能跨规则污染。
```yaml
# ❌ 危险：关键词 "agent" 太宽泛
ide:
  keywords:
    - "agent"    # 几乎所有 AI 扩展都包含

# ✅ 安全：精确扩展 ID
ide:
  ext_ids:
    - "GitHub.copilot-chat"
```

### 4. 大小写敏感性
`contains` 匹配**区分大小写**。如果需要大小写不敏感，添加变体：
```yaml
cmd_patterns:
  - type: contains
    value: "Claude"      # 大写 C
  - type: contains
    value: "claude"      # 小写 c
```

### 5. Regex 中的特殊字符转义
规则中正则表达式在 YAML 字符串内，`\` 需要双重转义：
```yaml
# ❌ 错误：YAML 解析后变成 \d+，Go 正则不识
version_regex: "version[= ]*(\d+\.\d+\.\d+)"

# ✅ 正确：YAML 解析后 `\\d` 变成 `\d`
version_regex: "version[= ]*([0-9]+\\.[0-9]+\\.[0-9]+)"
```

### 6. required 文件的缺失处理
当文件规则标记为 `required: true` 且路径不存在时，**整条规则不会产生任何检测结果**。这在多平台规则中尤其重要——某个平台的路径不存在时，需要确保其他平台的 `required` 路径可以命中。

### 7. 规则 name 冲突
两个规则使用相同的 `name`，引擎会将它们视为同一个 Agent 的不同检测维度，结果会合并去重。如果意图是不同 Agent，确保 `name` 唯一。
