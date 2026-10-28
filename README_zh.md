<div align="center">

# learn-agent-go

**用 Go 从零写一个 Claude Code 式的编程 Agent，每一课只讲一个概念。**

[![ci](https://github.com/cananoo/learn-agent-go/actions/workflows/ci.yml/badge.svg)](https://github.com/cananoo/learn-agent-go/actions/workflows/ci.yml)
[![Go](https://img.shields.io/badge/go-1.24+-00ADD8?logo=go)](go.mod)
[![License: MIT](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)

[English](README.md) | 简体中文

</div>

Agent 就是一个循环。工具、计划、子 Agent、技能、上下文管理，全都是在这个循环**外面**加东西，循环本身一行都不用改。

这个仓库分**六步**写出一个能干活的编程 Agent。每一步都是**一个可以直接运行的 Go 文件**，只比上一步多一个概念，所以你可以用 `diff` 一步步看清整个设计。没有框架，没有魔法：只用 Anthropic 官方 Go SDK 和标准库。

```go
for {
    resp := model(messages, tools)
    messages = append(messages, resp)
    if resp.StopReason != "tool_use" {
        return                          // 模型说完了
    }
    messages = append(messages, runTools(resp))  // 把工具结果喂回去
}
```

## 课程

| # | 课程 | 核心概念 | 行数 |
|---|------|----------|-----:|
| [s01](agents/s01_loop/main.go) | **Agent 循环** | 一个循环加一个 `bash` 工具，就已经是 Agent 了 | 141 |
| [s02](agents/s02_tools/main.go) | **工具** | 加能力 = 加 schema + 在分发表里加 handler，循环不变 | 270 |
| [s03](agents/s03_todo/main.go) | **计划** | `todo` 工具什么都不做，只让计划可见；久不更新就提醒 | 358 |
| [s04](agents/s04_subagent/main.go) | **子 Agent** | 同一个循环，空的 `messages`：读 30 个文件，只交回一段话 | 428 |
| [s05](agents/s05_skills/main.go) | **技能** | 两层加载：目录常驻提示词，完整 `SKILL.md` 按需读取 | 524 |
| [s06](agents/s06_compact/main.go) | **上下文压缩** | 上下文快满时：存档、总结、从一条摘要重新开始 | 644 |

每课配一篇讲解：[中文](docs/zh) · [English](docs/en)。

## 快速开始

```bash
git clone https://github.com/cananoo/learn-agent-go
cd learn-agent-go
cp .env.example .env        # 在 .env 里填上 key
go run ./agents/s01_loop
```

然后试试：

```
s01 >> 写一个 hello.go 打印前 10 个质数，然后运行它
s03 >> 给 hello.go 加一个 --verbose 参数，两种模式都测一下      （看它先列计划）
s04 >> 这个仓库里哪些包没有测试？用子 Agent 去查              （看主上下文保持干净）
s05 >> review 一下 agents/s01_loop/main.go                    （看它自动加载 code-review 技能）
```

> ⚠️ Agent 会在当前目录真实执行 shell 命令，请在临时目录或容器里跑。命令黑名单只是安全带，不是沙箱。

## 国内模型也能跑

官方 SDK 会读取 `ANTHROPIC_API_KEY` / `ANTHROPIC_AUTH_TOKEN` 和 `ANTHROPIC_BASE_URL`，所以只要服务商提供 Anthropic 兼容接口，代码一行不用改：

| 服务商 | `ANTHROPIC_BASE_URL` | `MODEL_ID` |
|---|---|---|
| Anthropic（默认） | 不设置 | `claude-opus-5-5`、`claude-sonnet-5-5`、`claude-haiku-5-5` |
| DeepSeek | `https://api.deepseek.com/anthropic` | 见服务商文档 |
| 月之暗面 Kimi | `https://api.moonshot.cn/anthropic` | 见服务商文档 |
| 智谱 GLM | `https://open.bigmodel.cn/api/anthropic` | 见服务商文档 |
| Ollama（本地） | `http://localhost:11434` | 任意支持工具调用的模型 |

接口地址来自各服务商的公开文档，不同模型的工具调用能力差别很大。如果你用某家跑通了（或者跑不通），欢迎提 issue，让这张表保持真实。

## 为什么要自己写一遍？

因为写过这个循环之后，所有 Agent 产品都不再神秘。你会在各种“规划模式”里认出 s03，在“深度研究”里认出 s04，在“规则 / 技能 / 记忆库”里认出 s05，在“上下文已压缩”的提示里认出 s06。自己的 Agent 出问题时，你也知道该拧哪个旋钮。

**为什么用 Go？** 消息结构是显式类型而不是字典，一眼看清；整个 Agent 编译成一个二进制；后面的并行工具、后台子 Agent 用 goroutine 写起来很自然。

## 怎么测试的

每一课的循环都用一个脚本化的假 Messages API（[internal/fakeapi](internal/fakeapi/fakeapi.go)）**离线测试**，不需要 key：

```bash
go test ./...
```

测试覆盖 Agent 真正容易出错的地方：并行工具结果必须放在同一条消息里返回、子 Agent 从空上下文开始且不能再派生子 Agent、路径不能逃出工作目录、压缩后从一条摘要重新开始，等等。

## 目录结构

```
agents/
  s01_loop/      循环 + bash
  s02_tools/     + 读 / 写 / 改文件，分发表
  s03_todo/      + todo 计划，提醒
  s04_subagent/  + task 工具，空上下文的子循环
  s05_skills/    + 技能目录 + load_skill
  s06_compact/   + 自动 / 手动压缩（完整版 Agent）
skills/          示例技能（go-testing、code-review）
internal/kit     环境变量、客户端、输出、REPL，不含 Agent 逻辑
internal/fakeapi 测试用的假 Messages API
docs/en, docs/zh 每课讲解
```

## 路线图

- [ ] s07 权限：危险工具执行前确认，白名单
- [ ] s08 流式输出
- [ ] s09 MCP：从外部服务器加载工具
- [ ] s10 用 goroutine 并行跑子 Agent

欢迎提想法和 PR。如果它帮你看懂了 Agent，点个 ⭐ 能让更多人看到。

## 致谢

灵感来自 Python 项目 [shareAI-lab/learn-claude-code](https://github.com/shareAI-lab/learn-claude-code)。本项目是独立的 Go 实现，课程设计不同，与 Anthropic 无关。

## 许可证

[MIT](LICENSE)
