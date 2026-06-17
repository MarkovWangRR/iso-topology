# 灵活几何体系统 — 开发与验收计划

> 对应技术方案：[flexible-geometry.md](flexible-geometry.md)。
> 本计划补充方案中未覆盖的集成面风险，并把"如何验收"具体化为
> 节点级与场景级两层端到端闸门。

## 一、方案低估的四道集成关卡

技术方案聚焦渲染层三层分离，但本库的下游消费者远不止 SVG 输出。
每个新形状落地都要穿过四道隐形关卡，这是真正的工程量所在，接口
设计必须把它们前置：

| # | 关卡 | 现状假设 | 新形状的破绽 | 接口对策 |
|---|---|---|---|---|
| 1 | 连接线锚点 | `anchorWorld` / `anchorRefineSilhouette` / `silhouetteHex` 全部假设矩形 bbox | 六棱柱的 `left-mid` 在哪？torus 的剪影裁剪怎么算？ | `ShapeProvider` 增加 `Silhouette(w,d,h)` 与 `Anchor(name)` 职责 |
| 2 | 顶面内容适配 | `fitTopContent` 假设矩形可用区 | 三棱柱顶面 / dome / H 类竖立面板（ContentAnchor 是侧立面） | 接口增加 `ContentRect(face)` —— 面内接矩形 |
| 3 | 布局求解器 | overlap 检查、group autosize、平台提升（h≤24）全吃 bbox | G 类阵列的 footprint 语义、E 类复合体的占地 | `Footprint()` 方案已有 ✓，需明确阵列/复合的定义并测试 |
| 4 | 契约面同步 | `validShapeList` / capabilities / dsl.schema / PROMPT_TEMPLATE 由 `CapabilityReport()` 派生 | agent 是一等用户：注册表不进 capabilities，agent 就写不出新形状 | shapeRegistry 成为 capabilities 的数据源；gen-docs 自动跟随 |

另一个方案未提的工程级风险：**确定性**。`SurfaceMap` 是 map，
遍历序随机 → golden 测试无序抖动。所有 def-id 生成和 face 遍历
必须显式排序，并由 M0 的双渲染字节相等测试兜底。

## 二、里程碑（每个独立可交付、可回滚）

### M0 · 验收基建先行（功能动工之前）

当前架构尚无 `[]Face` 中间表示，因此 M0 的不变量检查在
**渲染产物（SVG）层**实施，迁移后依然成立；Face 级深层不变量
随 M1 接口一起落地。

交付物：

1. **确定性测试**：每个 sample 双渲染，字节相等。专杀 map 序、
   随机 id 类回归。
2. **SVG 完整性测试**：所有坐标有限（无 NaN/Inf）、多边形点落
   在 viewBox 内（小容差）、def id 无未解析引用。
3. **像素回归工具 `tools/visreg`**：对每个 golden 按精确
   viewBox 截图（headless Chrome），与已提交基线 PNG 做像素
   diff（阈值 0.5%）；更新基线必须显式 `-approve`。这是
   "golden 字节没变"之外的第二道闸，专抓两类盲区：字节变了但
   视觉等价（应放行）、字节等价手段失效但视觉坏了（应拦截）。

**验收**：现有 6 形状 + 全部 sample 场景过三道检查，基线入库。

### M1 · 接口与 Box 平价迁移（方案 Phase 1）

- `Face` / `ShapeProvider` / `Surface` / registry 落地，接口比
  方案多三个方法：`Silhouette` / `Anchor` / `ContentRect`
  （一节的四道关卡入口）。
- Box 迁移为第一个 provider，**golden 字节零变化**。
- Face 级不变量库 `iso25d/geomtest` 随接口落地：多边形闭合、
  可见性与法线一致、ZOrder 拓扑正确、棱柱侧壁边沿严格
  ±tan30°、ContentAnchor 面存在且内容 bbox 在面内。

**验收**：全 golden 字节恒等 + visreg 零 diff + 不变量全绿。

### M2 · 用真新形状证明抽象（提前方案 Phase 4 的一部分）

> 调整理由：平价迁移无法暴露接口缺陷。必须用一个真正的新几何
> 在接口冻结前穿一遍全链路，M1/M2 之间允许 breaking change。

- `PrismShapeProvider`（A 类：diamond / hexprism / octprism
  一次拿下）。
- 穿透四道关卡：六棱柱连接线锚点 + 剪影裁剪、顶面内容内接
  矩形、layout overlap、capabilities/schema 自动出现新形状。

**节点级验收**：新形状 × 现有全部效果（gradient / pattern /
grain / wireframe / backglow / dash）矩阵 fixture，渲染单页
画廊截图，按视觉清单逐项评审。

**场景级验收**：真实场景 fixture（API 网关 hexprism、决策
diamond），validate exit 0；连接线箭头落剪影边的**程序化断言**
（解析 path 终点到剪影多边形距离 < ε）；Studio hover/pin 对新
形状正常（CDP 套件补断言）。

### M3 · Surface 扩展（方案 Phase 2）

- 多色标/径向渐变、多重描边、投影 Pattern、`style.faces` DSL
  + validator（未知 face 名 / fill kind 给 did-you-mean）。
- 旧字段在 resolver 转换为新格式；**旧 DSL golden 字节恒等**
  为硬门禁。

### M4 · Effect Pipeline（方案 Phase 3）

- Effect 接口 + 三个现有效果迁移 + innerGlow / blur / outline
  + 形状感知 backglow。
- 回归点：backglow 的 36px viewBox 预留改由 Effect 自报
  `Margin()`，不再硬编码。
- 验收：效果叠加顺序 fixture（同一节点 effects 两种顺序 →
  输出 z 序可证不同）；旧 effects 字段输出恒等。

### M5 · 形状家族滚动交付（方案 Phase 4/5 重排）

按方案自评的收益序 **F → B/D → H → C → G → E** 滚动，每族一个
独立提交，交付物固定四件套：provider + 不变量测试 + 节点级
矩阵 fixture + 场景级 fixture。

> F 类（虚线嵌套边界）提到最前：方案自己认定это"云架构图最
> 高频容器语义"，是 group 的直接升级，用户感知最强。

## 三、端到端验收体系（贯穿性闸门）

每个里程碑合入前过两层终审：

### 节点级（全自动，进 CI）

| 检查 | 工具 | 拦截什么 |
|---|---|---|
| golden 字节比对 | `go test`（现有） | 任何非预期输出变化 |
| 双渲染确定性 | M0 新增 | map 序 / 随机性回归 |
| SVG 完整性 | M0 新增 | NaN、越界、悬空引用 |
| Face 不变量 | M1 起 | 几何错误：开口多边形、错误剔除、斜率漂移 |
| 像素回归 | tools/visreg | 字节手段够不到的视觉回归 |

### 场景级（半自动，已验证两轮的方法论）

发一轮**多 agent 设计批判工作流**（本仓库已跑通两轮、修复 40+
算法缺陷的同款 harness）：4–6 个 agent 用新能力原创场景 →
渲染 → 看图自审 → 输出结构化缺陷清单（`algorithmic=true` 与
作者失误强制分离）。

通过标准：

1. 自评分中位数 ≥ 上一轮基线（当前 75）；
2. 新增 high 级算法缺陷 = 0；
3. 新形状在 agent 手里**能被正确使用**——误用率本身就是 DSL
   设计质量的验收指标，误用聚集处即文档/接口需返工处。

## 四、风险登记

| 风险 | 缓解 |
|---|---|
| 平价迁移字节不齐（浮点格式化路径变化） | M1 只改调用结构不改数学；`%.2f` 格式集中为常量 |
| SurfaceMap 无序 → golden 抖动 | 显式 sort 所有 map 遍历；M0 确定性测试兜底 |
| 接口发布后难改 | M2 用真新形状打穿全链路后再冻结；M1/M2 间允许 breaking |
| 范围蔓延 | 管道体积连接器、场景 overlay 已在方案 roadmap 圈定，本计划不接 |

## 五、节奏

M0+M1 一轮 → M2 一轮 → M3 / M4 各一轮 → M5 按族滚动。
每轮以"提交 + 三道自动闸全绿 + agent 终审达标"收口。
