<div align="center">

<img src="docs/assets/logo.jpg" alt="iso-topology logo — 文本 DSL 流入等距方块拓扑图" width="520">

# iso-topology — 代码即等距架构图

[![License: Apache 2.0](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)
[![Go 1.25](https://img.shields.io/badge/Go-1.25-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![Go Reference](https://pkg.go.dev/badge/github.com/MarkovWangRR/iso-topology.svg)](https://pkg.go.dev/github.com/MarkovWangRR/iso-topology)

**文本进，等距 SVG 出。AI agent 能生成、能校验、能 diff 的架构图。**

单一静态二进制 · 零运行时依赖 · 毫秒级渲染 · 35 个内置图标 · 全样例 golden 测试

[English](README.md) · 简体中文

</div>

---

iso-topology 是一个开源的 Go CLI 与库，把一种小巧的文本 DSL 渲染成
设计级的 2.5D **等距（isometric）SVG 架构图**。它是为 agent 而生的
diagram-as-code 工具：DSL 小到 LLM 一次提示就能产出，渲染前可被机器
校验（带"你是不是想写……"的修复建议），输出确定到可以和代码一起提交、
一起 diff。

Agent 优先，人类同样好用。

## 从这段文本……

```yaml
nodes:
  scene:
    shape: composite
    parts:
      - id: core                          # hero 锚定整个场景
        shape: rectangle
        geom: { w: 170, d: 170, h: 24 }
        icon: "iso://glyph/sparkles/7C5CFC"
        label: "AI Core"
        style:
          effects: { cornerRadius: 14, backglow: { color: "#A78BFA", radius: 46 } }
      - id: llm
        place: { behind: core, gap: 2.6 } # ← 永远写关系，不写坐标
        icon: "iso://glyph/chat"
        label: "LLM Gateway"
      # ……其余七个卫星，各一条 place 规则
```

## ……到这张图

![由 iso-topology 从 YAML 渲染出的等距 AI 平台架构图](docs/assets/ai-platform.png)

本 README 的每一张图都**完全由 `place` 关系和 `layout` 容器定位**——
没有一个手算坐标。完整源码：
[samples/topology/ai-platform/input.yaml](samples/topology/ai-platform/input.yaml)。

## 从这里开始——把这段话粘给 Claude

在一台全新的电脑上，你不应该读安装文档——agent 应该完成全部前置
准备，然后反过来教你工作流。把下面这段粘进 Claude Code（或任意
编码 agent），回来时你会得到装好的工具链、一张渲染完成的样例图、
和一份写给你的使用说明：

````markdown
Set up the iso-topology diagram toolchain on this machine, then teach
me how to use it. Work autonomously; only stop if something needs my
password or a decision only I can make. Reply in the language I use.

## 1 · Install (idempotent — skip whatever is already present)
- Ensure Go ≥ 1.25 (`go version`); if missing, install it with the
  system package manager (macOS: `brew install go`; Debian/Ubuntu:
  `sudo apt install golang-go`; otherwise https://go.dev/dl).
- `go install github.com/MarkovWangRR/iso-topology/cmd/isotopo@latest`
- `go install github.com/MarkovWangRR/iso-topology/cmd/isotopo-mcp@latest`
- Ensure `$(go env GOPATH)/bin` is on PATH for this session.
- Install the drawing skill so future sessions already know the
  workflow:
  `mkdir -p ~/.claude/skills/draw-iso-diagram && curl -sL https://raw.githubusercontent.com/MarkovWangRR/iso-topology/main/skills/draw-iso-diagram/SKILL.md -o ~/.claude/skills/draw-iso-diagram/SKILL.md`

## 2 · Verify with a real render
- `isotopo capabilities | head -20` must print JSON.
- Render the showcase sample into ./diagrams/hello:
  `curl -sL https://raw.githubusercontent.com/MarkovWangRR/iso-topology/main/samples/topology/ai-platform/input.yaml -o /tmp/hello.yaml && isotopo render /tmp/hello.yaml ./diagrams/hello`
- Open ./diagrams/hello/topology.html and tell me what I should see.

## 3 · From now on, whenever I ask for a diagram
- Read `isotopo capabilities` once per session; imitate the closest
  fixture from
  https://raw.githubusercontent.com/MarkovWangRR/iso-topology/main/docs/agent/SAMPLES.md
  and follow the visual rules in
  https://raw.githubusercontent.com/MarkovWangRR/iso-topology/main/docs/guides/scene-design.md
- Author YAML with layout/place relations ONLY — never hand-computed
  coordinates; connectors are always routing: orthogonal.
- Loop `isotopo validate <file>` until exit 0, then render into
  ./diagrams/<kebab-case-name>/ and give me the topology.html path.
- Keep the YAML next to the output as input.yaml; when I ask for
  changes, edit it and re-render the same folder.

## 4 · Finish by telling me
- three example requests that show off what this tool does well, and
- how I should phrase change requests so you can apply them precisely.
````

之后你只做两件事：**提需求**（"画我的 RAG 流水线，暗色，翡翠
accent，向量库是主角"）和**迭代**（"把缓存挪到网关右边"）——产物
永远落在 `./diagrams/<名字>/topology.html`。完整指南（怎么措辞、
怎么调试）见
[docs/getting-started/00-onboarding.md](docs/getting-started/00-onboarding.md)。

## 快速开始（手动路径）

```bash
# 安装（单一静态二进制，可直接放进 CI 镜像或 agent 容器）
go install github.com/MarkovWangRR/iso-topology/cmd/isotopo@latest

# 渲染上面那张 hero 图
curl -sLO https://raw.githubusercontent.com/MarkovWangRR/iso-topology/main/samples/topology/ai-platform/input.yaml
isotopo render input.yaml ./out
open ./out/topology.html        # SVG 与可编辑 DSL 并排
```

或者三行图先跑通，让自动布局包办一切：

```bash
echo 'user -> api -> db' > scene.d2
isotopo render scene.d2 ./out
```

## 让你的 agent 画图

iso-topology 讲契约，不讲感觉。agent 发现 DSL → 产出场景 → 拿到
机器可读的反馈 → 收敛，全程无人值守：

```bash
isotopo capabilities          # 机器可读的 DSL 清单（每会话读一次）
isotopo validate scene.yaml   # JSONPath 定位的问题 + 修复建议
isotopo render   scene.yaml out
```

带笔误的草稿跑 `validate` 的输出——按 `suggest` 改完重跑即可：

```json
{
  "issues": [
    {
      "severity": "error",
      "path": "nodes.scene.parts[0].shape",
      "message": "unknown shape \"cilinder\"",
      "suggest": "cylinder"
    }
  ]
}
```

退出码：`0` 干净 / `2` 仅警告 / `3` 有错误——可直接接入 CI。布局问题
同样被捕获：悬空的 `place` 引用、关系成环、解算后的重叠（精确到碰撞
的节点对）。

30 秒接入 Claude / Cursor / 任意 LLM——把下面这段话喂给模型：

```text
You can render iso architecture diagrams. Generate DSL using the
schema at docs/agent/schema/dsl.schema.json and the reference at
docs/agent/CAPABILITIES.md; imitate the fixture from
docs/agent/SAMPLES.md that best matches the task. Use
`isotopo validate <file>` to check before claiming done.
```

完整的实战系统提示词在
[docs/agent/PROMPT_TEMPLATE.md](docs/agent/PROMPT_TEMPLATE.md)——
其能力段由代码生成，永不过期。

本仓库还内置两种更深的集成：

- **MCP server**——`isotopo-mcp` 把 `iso_capabilities` /
  `iso_validate` / `iso_render` 作为 MCP 工具通过 stdio 暴露，
  Claude Code / Claude Desktop / Cursor 无需 shell 即可作画：
  `claude mcp add isotopo -- isotopo-mcp`。
  配置见 [docs/agent/MCP.md](docs/agent/MCP.md)。
- **Agent 技能**——[`skills/draw-iso-diagram`](skills/draw-iso-diagram/SKILL.md)
  是可安装的 Claude Code skill，编码了完整工作流（发现 → 模仿样例 →
  创作 → 校验循环 → 渲染）和画廊遵循的视觉质量规则：
  `cp -r skills/draw-iso-diagram ~/.claude/skills/`。

仓库根目录另有（同样是生成的）[`llms.txt`](llms.txt)，供生成式引擎
与 agent 爬虫自助发现。

## 画廊

### LLM 推理平台（暗色 · 分层流）

![暗色等距 LLM 推理平台架构图——客户端经服务网关到 GPU 池](docs/assets/llm-serving.png)

Chat 应用与 CLI 的请求流经服务网关——一块 `layout: grid` 的 hero
面板（路由 / 护栏 / 缓存 / 鉴权，每格一个青色图标 + 标题）——进入
后方的模型平面：GPU 池与模型注册表副本堆叠。
[源码](samples/topology/llm-serving/input.yaml)。

### RAG 流水线（暗色 · 双平面）

![暗色等距 RAG 架构图——摄取平面、向量数据库堆叠、服务平面](docs/assets/rag-pipeline.png)

后平面摄取与索引（文档 → ETL → 向量化），前平面服务
（应用 → 检索器 → LLM），共享向量库立于两者之间——ANN 查询走一条
虚线贝塞尔。[源码](samples/topology/rag-pipeline/input.yaml)。

### 训练算力（亮色 · 幽灵体积）

![等距条形图——各训练阶段 GPU 小时与虚线预算幽灵体积](docs/assets/training-compute.png)

一次训练的 GPU 小时去向：渐变柱体顶面带图标与时长，上方虚线线框
"幽灵"体积表示各阶段预算上限。
[源码](samples/topology/training-compute/input.yaml)。

### 平台电路板（亮色 · PCB / 蓝图）

![爆炸式等距电路板插画——浮空板上的斜线芯片](docs/assets/platform-board.png)

落地页 hero 级别的质感：三层板用 `place: {above}` 链式爆炸悬浮，
斜线填充的芯片由粗紫走线连接，虚线内嵌框、点阵纹理板、悬浮在主
芯片上方的线框幽灵体。
[源码](samples/topology/platform-board/input.yaml)。

### 身份流（白底 · 胶片颗粒排版风）

![单色颗粒质感身份图——人类委托 AI agent，agent 消费机器身份](docs/assets/identity-flow.png)

"平面广告"语域：白底上的近黑 `effects.grain` 颗粒物体、带药丸标
签的发丝连线、屏幕空间的粗体排版——横排由 `rightOf`+`behind` 配
合分轴 gap 实现屏幕水平。
[源码](samples/topology/identity-flow/input.yaml)。

### 三行 d2 的微服务（自动布局）

![由 d2 图源自动布局的等距微服务拓扑](docs/assets/microservice.png)

```d2
user:   User { shape: person }
api:    API Gateway
db:     Database { shape: cylinder }
user -> api: request
api  -> db:  write
```

完全不写位置——dagre（或 ELK）排图，iso-topology 升维到 2.5D。
[源码](samples/topology/microservice/input.d2)。

## 为什么是等距，为什么是这个工具

平面 2D 拓扑读起来是一排盒子；等距读起来是一个**系统**——纵深轴
一眼分开边缘 / 中间层 / 数据层，堆叠节点天然表达副本与高可用。
在 Figma 里手绘等距图，规模上限约十个元素，而且没人能 review diff。

|  | Mermaid | D2 | Figma / draw.io | **iso-topology** |
|---|---|---|---|---|
| 源格式 | 文本 | 文本 | 画布 | **文本（YAML / d2 / JSON）** |
| 视觉档次 | 流程图 | 流程图 | 设计级 | **设计级等距** |
| git 可 diff | ✓ | ✓ | ✗ | ✓ |
| agent 可发现 DSL（`capabilities`） | ✗ | ✗ | ✗ | ✓ |
| 渲染前校验 + 修复建议 | ✗ | ✗ | ✗ | ✓ |
| 无需手调坐标 | ✓ | ✓ | ✗ | ✓（`place`/`layout` 求解器） |
| 离线单二进制 | ✗（浏览器/node） | ✓ | ✗ | ✓ |

## 两种输入模式

| 路径 | 强项 | 适用 |
|---|---|---|
| `.d2` 图源 | dagre / ELK 自动布局 | agent 由图数据生成拓扑、动态场景 |
| `.yaml` 组合 | 声明式构图——`layout` 容器 + `place` 关系，零手算坐标 | 设计师级场景、营销视觉、信息图 |

两者收敛到同一文档模型与输出结构。参见
[d2 参考](docs/reference/dsl-d2.md) 与
[YAML 参考](docs/reference/dsl-yaml.md)。

## 能力清单

- **23 种 d2 形状**映射到等距原语（rectangle、cylinder、cloud、
  person、hexagon、queue、oval……）
- **声明式定位**：`layout: {mode: row|column|grid}` 容器与
  `place: {rightOf|inFrontOf: sibling}` 关系——求解器算坐标、
  校验引用、警告重叠
- **8 个组合原语**：`group`、`stack`、`layout`、`place`、
  `canvas.grid`、`annotation`、`connector`（orthogonal / bezier）、
  图标
- **35 个内置图标**：18 个 AI / 大数据 glyph
  （`iso://glyph/gpu`、`model`、`agent`、`vector`、`lake`……）任意
  着色，外加品牌徽章（`iso://brand/kafka`……）
- **面级样式**：逐面渐变、dropShadow、backglow、网格/点阵纹理、圆角
- **设计系统**：`theme.presets` 命名风格预设——部件一句 `preset: <name>`
  引用（YAML 锚点的 JSON 安全、可校验替代品）
- **两级输出**：整场 SVG + 逐元素独立 SVG

机器可读全量清单：`isotopo capabilities`。

## 输出结构

```
out/
├── topology.svg              整个场景
├── topology.html             SVG 与可编辑 DSL 源并排
├── topology.<yaml|d2|json>   源文件副本
└── nodes/
    ├── _index.html           元素画廊
    ├── <id>.svg              独立等距元素
    ├── <id>.html             嵌入片段
    └── <id>.yaml             可二次渲染的 DSL 片段
```

## 作为 Go 库使用

```go
import isotopo "github.com/MarkovWangRR/iso-topology"

doc, _ := isotopo.Parse(yamlBytes)
svg := isotopo.RenderWithCanvas(doc.Scene(), doc.Theme, doc.Canvas, doc.Annotations)
```

完整 API：[docs/reference/cli.md](docs/reference/cli.md)。

## FAQ

**和 Mermaid / D2 有什么区别？**
Mermaid 和 D2 产出平面流程图；iso-topology 产出设计级 2.5D 等距
场景——那种你原本要在 Figma 里花一下午手绘的发布配图——同时保持
文本源、可 git diff。它也是三者中唯一为 agent 设计的：
`capabilities` 契约、JSON Schema 本地 lint、带修复建议的 `validate`。

**ChatGPT / Claude / 我的 agent 能直接画吗？**
能——这就是设计中心。agent 读
[`isotopo capabilities`](docs/agent/CAPABILITIES.md)（或
[JSON Schema](docs/agent/schema/dsl.schema.json)），产出 YAML，再用
`isotopo validate` 的 JSONPath 问题清单自我纠错。即插即用的系统
提示词在 [PROMPT_TEMPLATE.md](docs/agent/PROMPT_TEMPLATE.md)；
MCP 用户直接 `claude mcp add isotopo -- isotopo-mcp`。

**必须手动摆位置吗？**
不用。场景由 `place` 关系（"rightOf: gateway, gap: 2"）和 `layout`
容器（行/列/网格 + 底座自动包裹）组成，确定性求解器把关系变成坐标；
`offset` 仅作微调增量。本 README 里每张图都是零坐标的。

**渲染需要浏览器、字体或网络吗？**
不需要。一个静态 Go 二进制，无 CGO、无系统字体、无网络。输出是
确定性的——同样输入永远同样字节——这也是 golden 测试和干净 git
diff 的基础。

**可以商用吗？**
可以——Apache 2.0，含渲染产物。内置品牌徽章是原创字母标识，
不是商标 logo 的复制品。

## 文档

按用途组织——总索引见 [docs/README.md](docs/README.md)。

- **入门：**[教程](docs/getting-started/01-install.md) · [配方](docs/agent/RECIPES.md) · [场景设计](docs/guides/scene-design.md) · [排障](docs/guides/troubleshooting.md)
- **参考：**[CLI 与库](docs/reference/cli.md) · [YAML DSL](docs/reference/dsl-yaml.md) · [d2 DSL](docs/reference/dsl-d2.md) · [样式/主题](docs/reference/dsl-theme.md) · [输出结构](docs/reference/output-layout.md)
- **Agent 集成：**[CAPABILITIES.md](docs/agent/CAPABILITIES.md) · [PROMPT_TEMPLATE.md](docs/agent/PROMPT_TEMPLATE.md) · [SAMPLES.md](docs/agent/SAMPLES.md) · [dsl.schema.json](docs/agent/schema/dsl.schema.json) · [MCP](docs/agent/MCP.md) · [skills/](skills/README.md)
- **设计：**[为什么是等距](docs/concepts/why-isometric.md) · [扩展](docs/guides/extending.md)

## 路线图

- 曲面形状（圆柱/球体侧面）的纹理支持
- `place` 分轴间距；`ring` 环形布局模式
- 更多领域图标包
- 渲染期视觉 lint（重叠/裁切诊断输出 JSON）

## 状态

单作者项目，迭代很快。依赖请钉 tag；`oss.terrastruct.com/d2` 锁定
`v0.7.1`。

## 贡献与晒图

欢迎 issue 和 PR——提交前跑 `go test ./...`；
`samples/*/*/expected.svg` 是 golden 文件，守护渲染管线不漂移。

**画出了好东西？** 开 issue 附上场景与 SVG——优秀作品会被画廊收录。
如果 iso-topology 帮你省下了一个 Figma 下午，点个 ⭐ 让更多人找到它。

## 许可证

Apache License 2.0——见 [LICENSE](LICENSE)。
