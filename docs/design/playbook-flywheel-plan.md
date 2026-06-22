# Playbook 设计资产飞轮 — 开发计划 (P0–P6)

> 目标:在 `samples/playbook/` 下,每个设计风格一个自包含子目录;搭一条双向飞轮——
> **(A) 设计手册辅助 agent 设计**、**(B) 从 infra 公司 landing page 反向蒸馏沉淀风格**。
> 全程闭环、可验收。本文是可执行的开发蓝图。

## 0. 不变量与全局约定(先定死)

- **不改 isotopo 渲染器 / DSL**。风格通过 `playbook apply` 预处理,把"结构+role"编译成**标准 isotopo YAML**(只用现有 `palette/effects/stroke/text`)。`role/material/token` 只活在 playbook 层。
- **实现**:Go 子命令 `isotopo playbook …`(`cmd/isotopo/playbook.go` + `internal/playbook` 包),复用 isotopo 库;LLM/视觉走 `internal/llm`(provider 抽象)。
- **复用清单(已勘探)**:
  - 库:`isotopo.Parse`、`isotopo.Validate`、`isotopo.RenderDocument`、`isotopo.LoadInput`;类型 `Style/Palette/Effects/Theme/FaceGradient`;`ResolveStyle`。
  - 颜色:`validate_visual.go` 的 `parseHex/lumOf/contrastRatio`;`planview.go` 的 `parseHex6/planTint/planIsDark`(HSL 派生在此之上补 `internal/playbook/color.go`)。
  - 图标:`iso25d.ResolveBrandIcon(uri)`,支持 `iso://…/light` 与 `iso://…/RRGGBB` 着色 + base64 内联 → icon treatment 即"选后缀"。
  - CLI:`cmd/isotopo/main.go` 的 `switch args[0]` + `usage()/os.Exit`。
  - golden:`golden_test.go` 扫 `samples/{node,topology}/*` 比对 `expected.svg`(`-update` 接受)→ 扩到 `samples/playbook/*`。
  - 生成物:`tools/gen-docs` 模式 → `playbook index` 生成 `samples/playbook/INDEX.json`。
  - skill:`skills/draw-iso-diagram/SKILL.md` 就在库里,可改。
- **LLM env**:`PLAYBOOK_LLM_PROVIDER`(`openai`|`minimax`)、`PLAYBOOK_LLM_BASE`、`PLAYBOOK_LLM_KEY`、`PLAYBOOK_LLM_MODEL`、`PLAYBOOK_VISION_MODEL`;minimax adapter 用 `MINIMAX_API_KEY` + `https://api.minimaxi.com`。**无 key → LLM 相关测试 `t.Skip`**。
- **确定性 vs LLM 的验收分界**:确定性层(color/kernel/apply/index/search)走 **golden + 单测**;LLM 层(extract/synthesize/judge)走 **schema 合法 + 阈值 + 收敛**,key-gated。

## 1. 目录与产物

```
samples/playbook/
  _exemplar.yaml          # 共享标准结构图(节点带 role,无样式)— 飞轮的"靶子"
  _roles.yaml             # 通用 role 本体(单一真相,被 skill/校验引用)
  INDEX.json              # 生成物:agent 检索目录(从各 meta.yaml 汇总)
  <style>/
    manual.yaml           # 可执行手册:tokens/material/geometry/roles/edge/canvas/icon
    meta.yaml             # name/title/tags/domain/mood/palette/provenance/trust/confidence/version
    source/{landing.png, source.md}
    preview/{exemplar.svg, exemplar.png, swatches.svg}   # exemplar.svg 进 golden
    distill/{extracted.json, iterations.jsonl, report.md}
```

## 2. 架构与两条流

```
                ┌──────────── Flow B (反向蒸馏 / 逆渲染闭环) ────────────┐
 landing.png →  extract(vision) → synthesize(chat) → manual.yaml
                                         │  ┌── render(_exemplar) → preview ──┐
                                         └──┤   judge(vision: preview vs landing)│ refine↺
                                            └── score≥target? → gate→写盘→index ─┘
                ┌──────────── Flow A (手册辅助 agent 设计) ─────────────┐
 intent →  playbook search → 选 style → agent 写 structure(role) → playbook apply → isotopo render → judge 自检
```
**飞轮闭合**:B 产 `manual.yaml` → A 消费。agent ReAct 里 `search` 落空 → 触发 `distill`(B)→ 再 `apply`(A)。

## 3. 阶段依赖

```
P0 ─ P1 ─ P2 ─ P3            (P0..P3 = 纯确定性闭环,零 LLM,先跑死)
          └── P4 ─ P5 ─ P6   (P4..P6 = 叠 LLM 蒸馏 + 飞轮合龙)
```

---

# P0 — 脚手架 + schema + 基准手册

**目标**:把骨架、数据契约、靶子立起来,零逻辑。

**交付**
- `internal/playbook/schema.go`:`Manual{Name,Extends,Tokens,Material,Geometry,Roles,Edge,Canvas,Icon}`、`Meta{...}` Go struct(yaml+json tag);`LoadManual/LoadMeta`。
- `internal/playbook/schema_test.go`:struct ↔ yaml 往返无损。
- `cmd/isotopo/playbook.go`:`case "playbook"` 子分发骨架(`lint` 先实现,其余 stub)。
- `samples/playbook/_roles.yaml`:通用 role 本体 `hero/surface/source/sink/store/gateway/group/accent`(每个带一句语义)。
- `samples/playbook/_exemplar.yaml`:固定结构图(sources→ingestion→hero→consumption+governance,节点标 role,无样式)。
- `samples/playbook/lustre/{manual.yaml,meta.yaml}`:**手写**的 StarRocks 审美基准(material 旋钮 = 之前调好的常数)。
- `isotopo playbook lint <style>`:校验 manual/meta schema、token 引用闭合、role 都在 `_roles.yaml`。

**验收**
```
isotopo playbook lint lustre        # exit 0, 0 error
go test ./internal/playbook/ -run Schema   # 往返无损
```

---

# P1 — 皮肤内核 + apply + render(确定性核心)

**目标**:`role + manual → 标准 isotopo YAML → SVG`,完全确定性,可 golden。

**交付**
- `internal/playbook/color.go`:`parseHex/toHSL/fromHSL/lighten/darken/mix`(复用现有 `parseHex`,补 HSL)。
- `internal/playbook/kernel.go`:`DeriveFaces(base, Material) (top,leftGrad,rightGrad)` —— 顶=`shade(base,topDL)`、受光侧/背光侧按 `litSideDL/shadeSideDL`、各侧叠 `aoDL` 竖直渐变;圆角时置 `faceSplit`。
- `internal/playbook/apply.go`:`Apply(structure []byte, m *Manual) ([]byte, error)` —— 逐节点 `role→base token→DeriveFaces→emit palette/effects/stroke/text`;icon treatment→`iso://…/RRGGBB|light` 后缀;edge/canvas 按手册。输出**纯标准 isotopo YAML**。
- CLI:`playbook apply <style> <structure.yaml> [-o]`;`playbook render <style> [structure]` = apply+`RenderDocument`,写 `preview/exemplar.svg|png` + `swatches.svg`(每 role 一个盒)。
- golden:`golden_test.go` 扩扫 `samples/playbook/<style>/`,渲染 `_exemplar` 比对 `preview/exemplar.svg`。
- `internal/playbook/kernel_test.go`:`DeriveFaces` 单测(已知 base→已知三面)。

**验收**
```
isotopo playbook render lustre                 # 0 error/overlap;生成 preview/swatches
go test ./ -run TestGolden/playbook            # 逐字节稳定
go test ./internal/playbook/ -run Kernel       # 派生数值正确
```
- swatches 含 `_roles.yaml` 每个 role;同输入两次渲染逐字节相同。

---

# P2 — 索引 + 检索(agent 可发现性的数据层)

**目标**:把风格做成可被一个 token 选中、可被 agent 检索的目录。

**交付**
- `playbook index`:扫所有 `<style>/meta.yaml` → 生成 `samples/playbook/INDEX.json`(单一真相);校验 preview 存在。
- `playbook search <query> [--facet k=v] [--image png]`:对 INDEX 做 facet 过滤 + 关键词打分(P2 用关键词/标签;embedding 留 P5 的 `--image`)。**输出契约**(喂下一步):
  ```jsonc
  [{ "style","why","trust","confidence",
     "roles":[...],                         // 该风格支持的 role 词汇
     "preview":"…/exemplar.png",
     "apply":"isotopo playbook apply <style> <structure.yaml>" }]
  ```
- `playbook list`:紧凑菜单(name + 一句话 + trust + 预览路径)。
- `internal/playbook/index_test.go`、`search_test.go`。

**验收**
```
isotopo playbook index && test -f samples/playbook/INDEX.json
isotopo playbook search "clean blue data" --facet domain=data-platform   # 命中 lustre/…,按相关度排序,含 roles/preview/apply
```
- INDEX 覆盖全部风格;search 输出含 `roles/preview/apply` 三件套(schema 校验)。

---

# P3 — agent ReAct 集成(Flow A 端到端 + 可发现性验收)

**目标**:让 agent 在画图循环里**知道并真的去查** playbook,端到端跑通 A,并把"是否去查了"做成断言。

**交付**
- 改 `skills/draw-iso-diagram/SKILL.md`:插入**选风格 gate**(作结构前先 `playbook list/search`)、通用 role 本体表、**兜底**(无匹配→`default`/`lustre`)、`--image` 分支(有参考图→视觉检索,miss→提示 distill)。
- `samples/playbook/default/`:中性默认风格(永不裸奔)。
- `llms.txt`/`docs/agent`:新增 "Playbook registry" 入口(gen-docs 顺带产出)。
- **可发现性 e2e 测试** `test/affordance_e2e.sh`:跑一遍"给意图→agent 产图"流程,**断言 transcript 内出现** `playbook search`、结构节点带 `role:`、最终执行了 `playbook apply`。
- Flow A 端到端脚本:`search → 写新结构 → apply → render → judge`(judge 在 P4 后接;P3 先用结构合法性 + 人工眼检占位)。

**验收**
```
bash test/affordance_e2e.sh        # 断言:search 被调用 / 节点有 role / apply 被执行 —— 全中
isotopo playbook apply lustre samples/playbook/_demo.yaml | isotopo validate -   # 0 error
```
- "agent 是否查了 playbook" = 可 fail 的断言(无 LLM key 时 judge 项 skip,search/apply 断言照跑)。

---

# P4 — LLM 客户端 + 抽取 + 合成(Flow B 1–2)

**目标**:从 landing 截图抽出设计参数,合成 schema 合法的手册。

**交付**
- `internal/llm/client.go`:provider 抽象 `Chat(prompt) string`、`Vision(prompt, imgs...) string`;adapter:`openai`(`/v1/chat/completions`,image_url)、`minimax`(`api.minimaxi.com`,`MINIMAX_API_KEY`)。env 配置;无 key 报可识别错误。
- `internal/playbook/extract.go`:`Extract(img) Extracted`(vision + JSON schema 约束:palette/surface·accent·ink、shadow、radius、fonts、mood、icon treatment);CV 兜底取准 hex(复用 `parseHex6`)。
- `internal/playbook/synthesize.go`:`Synthesize(Extracted) *Manual`(chat;few-shot 用手写 `lustre`/`snowflake` 当范例)。
- CLI:`playbook distill <style> --source <img> --iters 0`(只跑抽取+合成,不进闭环)。
- 测试:`extract_test.go`/`synthesize_test.go`,**key-gated**;无 key 用 fixture(录制的样例响应)断言解析与 schema。

**验收**(有 key)
```
isotopo playbook distill snowflake --source samples/playground/snowflake/*.png --iters 0
isotopo playbook lint snowflake          # 产出的 manual schema 合法
isotopo playbook render snowflake        # exemplar 0 error
```

---

# P5 — 逆渲染判官闭环(Flow B 3–4)+ 视觉检索

**目标**:用我们自己的渲染器当正向模型,判官当 loss,迭代逼近源设计;落盘门控;`--image` 检索 + miss→distill。

**交付**
- `internal/playbook/judge.go`:`Judge(preview, source) (score, critique)`(vision 比对,prompt 判**设计语言神似**非像素对齐)。
- `internal/playbook/refine.go`:`Refine(m, critique) *Manual`(chat 按批评调 material/tokens)。
- `internal/playbook/distill.go`:`distill→synthesize→[render→judge→refine]×iters` 直到 `score≥target` 或耗尽;写 `distill/iterations.jsonl`+`report.md`;`meta.yaml{trust:auto, confidence:score, provenance}`;`playbook index` 自动纳入。
- `search --image png`:对 INDEX 的 preview 做视觉 embedding 最近邻(共用 judge 的视觉模型);**miss→返回"建议 distill"信号**(P3 skill 分支据此触发)。
- 测试:`distill_test.go` key-gated,断言**分数逐轮非降**且收敛/达标;`gate_test.go` 断言 auto 产物 trust/confidence 正确。

**验收**(有 key)
```
isotopo playbook distill databricks --source samples/playground/databricks/*.png --iters 4 --target 75
# iterations.jsonl 分数递增;终分≥75;meta.trust=auto;INDEX 自动含 databricks
isotopo playbook search --image <某截图>      # 命中对应风格 / 陌生图→建议 distill
```

---

# P6 — 飞轮合龙 + CI + 文档

**目标**:B→A 全自动跑通;CI 双轨门控;文档/skill/记忆收口。

**交付**
- `test/flywheel_e2e.sh`:**B→A 一条龙**——distill 某 landing → index → agent `search` 命中 → 写**新**结构 → apply → render → judge≥阈值。无人工干预。
- CI:确定性 golden/单测**必过**;LLM 项 **key-gated**(CI 无 key 自动 skip,不抖)。
- gen-docs:产 `docs/agent/PLAYBOOK.md`(给 agent 的 registry 用法)+ 把 `INDEX.json`、role 本体纳入 `llms.txt`。
- `skills/draw-iso-diagram/SKILL.md` 终稿(gate+role+兜底+image→distill 全含)。
- 记忆:写一条 `feedback` —— "画图先查 playbook 注册表(`search→选 style→role 作结构→apply`);风格沉淀走 distill 飞轮"。

**验收(总)**
```
bash test/flywheel_e2e.sh          # B→A 全跑通,两次 judge 均≥阈值,产物落盘+INDEX 更新
go test ./...                      # 全绿(LLM 项 skip 或通过)
```
- **Definition of Done**:上面一条命令无人工干预跑完;`samples/playbook/` 至少 3 个 blessed + ≥1 个 auto 蒸馏风格;agent e2e 断言其确实查并应用了 playbook。

---

## 风险红线(贯穿全程)
- **IP/授权**:只提取**风格参数(色值/圆角/阴影数学=不可版权事实)**,`provenance` 记来源 + "style-reference,不抄素材/logo"。验收前提。
- **平面→iso 鸿沟**:judge prompt 判"设计语言神似",否则不收敛。
- **schema 天花板**:蒸馏 match 不上的点 → backlog "内核加旋钮",驱动 `material` 演进(不扩 DSL)。
- **LLM 抖动**:确定性层吃 golden;LLM 层吃阈值+收敛+schema,key-gated。
