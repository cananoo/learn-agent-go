# s05 · 技能（Skills）

> 知识分两层：一行目录常驻，完整说明按需加载。

[← s04](s04-subagents.md) · [代码](../../agents/s05_skills/main.go) · [下一课：上下文压缩 →](s06-compaction.md)

## 问题

你希望 Agent 按团队的习惯做事：测试怎么写、代码怎么审、版本怎么发。全塞进系统提示词，就意味着每次请求都要为一万 token 的说明付费，而这些知识一周才用一次。系统提示词越长，每一部分分到的注意力也越少。

## 思路

像人用书架一样加载知识：先看书脊，需要哪本再抽哪本。

```
第 1 层 · 常驻系统提示词（便宜）:
    Available skills:
    - go-testing: Write or fix Go tests - table-driven tests, ...
    - code-review: Review a diff or a file for bugs ...

第 2 层 · 通过工具按需加载（完整内容）:
    load_skill("go-testing")  →  <skill name="go-testing"> ...完整 SKILL.md... </skill>
```

## 一个技能就是一个文件夹

```
skills/
  go-testing/
    SKILL.md
  code-review/
    SKILL.md
```

```markdown
---
name: go-testing
description: Write or fix Go tests - table-driven tests, subtests, t.TempDir, httptest fakes
---

# Writing Go tests
1. Find the existing style first ...
```

`description` 是最重要的一行：模型决定要不要加载这个技能时，只能看到它。要写清楚**什么时候**用，而不只是它是什么。

## 代码

解析不需要 YAML 库，按 `---` 切开就行：

```go
rest, ok := strings.CutPrefix(text, "---\n")
front, body, ok := strings.Cut(rest, "\n---")
```

`Catalog()` 把第 1 层渲染进系统提示词，`load_skill` 工具返回第 2 层。名字写错时，错误信息里会附上目录，模型能自己纠正。

## 为什么放在工具结果里，而不是系统提示词里？

会话中途改系统提示词，会让它后面所有内容的 prompt cache 失效。工具结果和其他消息一样追加在末尾：对缓存友好，而且正好出现在模型需要它的位置。

## 动手试试

```bash
go run ./agents/s05_skills
s05 >> review 一下 agents/s01_loop/main.go
```

它应该先调用 `load_skill("code-review")`，再按清单逐项检查。加一个你自己的技能文件夹，重启即可，不用改代码。把 `SKILLS_DIR` 指向别处，就能用共享的技能库。

## 和 s04 相比改了什么

- `skillSet` 及其 `Catalog()`、`Load()`，大约 80 行。
- 目录拼进系统提示词，多一个工具。
