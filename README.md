# AI Asset Discovery

[![Go Version](https://img.shields.io/badge/Go-1.26.1+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

AI Asset Discovery 是一款轻量级的 **AI 智能体资产清点工具**，能够自动扫描系统中的 AI 编程助手、IDE 扩展、桌面AI应用、Agent 框架及 LLM SDK，并提取每个 Agent 的能力（Skills）、配置文件等元信息。

## ✨ 特性

- **多维度检测**：进程指纹、文件系统、IDE 扩展、技能文件四维扫描
- **跨平台**：Linux / macOS / Windows 全平台支持（路径自动适配，进程扫描三平台均已实现）
- **规则驱动**：YAML 定义的检测规则，易于扩展和定制
- **置信度分级**：`confirmed`（确认）/ `possible`（可能）/ `ghost`（痕迹）三级置信度
- **Skill 发现**：自动发现 Agent 的技能文件并提取工具、参数、提示模板
- **零误报设计**：过滤自身进程、Shell 包装器等干扰项

## 🚀 快速开始

### 编译

```bash
# 要求 Go 1.26.1+
git clone https://github.com/your-org/ai-asset-discovery.git
cd ai-asset-discovery
CGO_ENABLED=0 go build -o discovery ./cmd/discovery/
```

### 运行

```bash
# 使用默认规则扫描并输出 JSON 结果
./discovery --rules rules/ --pretty=false

# 输出到文件
./discovery --rules rules/ --output result.json

# 带格式化输出（默认）
./discovery --rules rules/
```

### 命令行参数

| 参数 | 说明 | 默认值 |
|------|------|--------|
| `--rules` | 规则文件或目录路径 | `rules` |
| `--output` | JSON 结果输出文件路径 | 标准输出 |
| `--pretty` | 是否格式化 JSON 输出 | `true` |

## 📊 输出格式

```json
{
  "summary": {
    "total_agents": 5,
    "confirmed_agents": 1,
    "possible_agents": 3,
    "ghost_agents": 1,
    "total_skills": 12,
    "by_type": {
      "process": 2,
      "ide_extension": 1,
      "file": 1,
      "package": 1,
      "binary": 1,
      "probe": 1
    }
  },
  "agents": [
    {
      "name": "cline",
      "display_name": "Cline",
      "confidence": "confirmed",
      "asset_type": "ide_extension",
      "process": {
        "pid": 12345,
        "name": "code",
        "cmdline": "/usr/share/code/code --extensionDevelopmentPath=...",
        "cwd": "/home/user/project",
        "executable": "/usr/share/code/code"
      },
      "skills": [
        {
          "name": "code-review",
          "description": "Review code changes",
          "file_path": "~/.cline/skills/review.md",
          "tools": ["Bash", "Read", "Edit"],
          "parameters": { "depth": "full" }
        }
      ]
    }
  ]
}
```

## 📋 支持的检测类别

| 类别 | 说明 | 示例 |
|------|------|------|
| `cli_agent` | 命令行 AI Agent | Claude Code, Gemini CLI, Aider, Codex |
| `ide_extension` | IDE / 编辑器扩展 | GitHub Copilot, Cline, Continue, Tabnine |
| `desktop_app` | 桌面 AI 应用 | ChatGPT Desktop, Windsurf |
| `desktop_assistant` | 桌面 AI 助手 | WorkBuddy, Kimi, 豆包 |
| `browser_agent` | 浏览器 AI Agent | Manus, Fellou, Genspark |
| `web_tool` | Web 端 AI 工具 | Bolt.new, Replit Agent |
| `agent_framework` | Agent 开发框架 | LangChain, AutoGen, CrewAI, Dify, Coze |
| `sdk_detection` | LLM SDK 依赖检测 | 通用 LLM SDK 痕迹检测 |

## 📝 规则编写指南

规则以 YAML 定义，每条规则描述一个 AI Agent 的检测方式。完整的规则编写指南请参阅：

- **[规则编写与使用指南 (docs/rule-guide.md)](docs/rule-guide.md)** — 字段详解、匹配逻辑、实战案例、常见陷阱
- **[项目架构与流程文档 (docs/architecture.md)](docs/architecture.md)** — 架构图、扫描流程、时序图、扩展点

### 快速示例

```yaml
- name: claude-code                    # 唯一标识符（小写、短横线连接）
  display_name: "Claude Code"          # 显示名称
  category: "cli_agent"               # 分类
  min_confidence: possible            # 最低置信度
  paths:
    - path: ~/.claude-code
    - path: ~/.claude
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

> 完整规则语法、全部字段说明及最佳实践见 [docs/rule-guide.md](docs/rule-guide.md)。

## 🏗️ 项目架构

```
.
├── cmd/discovery/main.go        # CLI 入口
├── docs/
│   ├── rule-guide.md            # 规则编写详细指南
│   └── architecture.md          # 架构、流程、时序图文档
├── internal/
│   ├── config/config.go         # 配置与路径解析
│   ├── discovery/engine.go      # 扫描引擎（编排各 Scanner）
│   ├── ide/scanner.go           # IDE 扩展扫描器
│   ├── model/
│   │   ├── rule.go              # 规则数据结构定义
│   │   └── types.go             # Agent / Skill 等数据类型
│   ├── rule/loader.go           # YAML 规则加载器
│   ├── scanner/
│   │   ├── filesystem.go        # 文件系统扫描器
│   │   ├── process.go           # 进程扫描器（跨平台接口）
│   │   ├── process_linux.go     # Linux /proc 实现
│   │   ├── process_darwin.go    # macOS ps 实现
│   │   ├── process_windows.go   # Windows 实现
│   │   ├── package.go           # 包管理器扫描器
│   │   ├── binary.go            # PATH 二进制扫描器
│   │   └── probe.go             # 命令探测扫描器
│   ├── skill/discoverer.go      # Skill 发现器
│   ├── platform/paths.go        # 平台路径工具函数
├── rules/agents.yaml            # 检测规则（50+ Agent）
└── test/containers/             # Docker 容器化端到端测试
    ├── run-tests.sh
    ├── dockerfiles/
    └── fixtures/
```

> 详细架构说明、扫描流程与 Mermaid 时序图见 [docs/architecture.md](docs/architecture.md)。

## 🧪 测试

```bash
# 运行单元测试
go test ./...

# 运行容器化端到端测试
cd test/containers && bash run-tests.sh
```

## 📄 License

MIT License

## 🤝 贡献

欢迎提交 Issue 和 Pull Request。新增 Agent 检测规则请直接修改 `rules/agents.yaml`，格式参考上方规则编写指南。
