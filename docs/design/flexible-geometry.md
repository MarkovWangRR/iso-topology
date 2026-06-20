# 灵活几何体系统技术方案

## 一、背景与现状问题

### 1.1 当前架构

iso-topology 的渲染链路如下：

```
DSL (YAML/d2)
  → ResolveStyle (theme.go)         # 4层样式合并
  → Flatten (flatten.go)            # DSL → ConvertOpts 扁平结构体
  → Convert2DTo25D (shapes.go)      # switch 分发到各 Render 函数
  → RenderIsoBox / RenderIsoCylinder / ...  # 各自独立的字符串拼接
  → strings.Builder → SVG string
```

### 1.2 根本限制

**限制一：形状集合封闭**

`Convert2DTo25D` 是一个 `switch` 枚举，所有未知 shape 退化为 `RenderIsoBox`。无扩展点，无法从外部注册新形状，无法组合既有形状。

**限制二：每种形状各自孤立**

`IsoBoxOpts`、`IsoPersonOpts`、`IsoCylinderOpts` 是三套独立结构体。效果能力不对等：
- `cloud` 不支持 pattern
- `person` 不支持 backglow  
- `cylinder` 不支持 grain

原因是效果逻辑分散写在各渲染函数内部，没有统一管线。

**限制三：视觉效果不可叠加，顺序硬编码**

当前效果清单及其限制：

| 效果 | 当前限制 |
|------|---------|
| 渐变 | 仅线性，仅 5 个方向 |
| 阴影 | 单层，不能多重 |
| backglow | 固定圆形模糊，不感知形状轮廓 |
| pattern | 仅 hatch/dots，不能透视映射到面 |
| grain | 全局滤镜，不能 per-face |
| stroke | 不支持内描边、多重描边 |

所有效果的 Z 层顺序由代码写入顺序决定，用户无法干预。

**限制四：几何与外观耦合**

`RenderIsoBox` 在同一函数内同时负责：计算投影坐标、写多边形、写渐变 def、写 pattern、写文字。几何和渲染逻辑无法独立复用。

**限制五：输出是死字符串**

渲染产物是 `strings.Builder` 拼出的字符串。后处理层（`inject.go`）用字符串索引搜索插入点。无法在生成后重新组合样式，无法做响应式主题切换。

### 1.3 能力天花板

以下效果在当前架构下**完全不可能实现**，不是参数调节问题，而是结构性缺失：

- 发光的圆柱（backglow 只在 box family 实现）
- 贴图映射到等轴测面（透视变形的纹理）
- 布尔运算形状（带缺口的盒子、穿孔的平台）
- 多重描边（外描边 + 内高光线）
- 形状感知的阴影（阴影轮廓跟随形状）
- 节点状态变体（default / highlighted / faded）
- 自定义轮廓形状（非预设的异形节点）

---

## 二、目标

设计一套**几何与外观彻底分离**的渲染架构，使得：

1. **任意形状可享受任意效果组合**——效果管线与几何无关
2. **形状可从外部扩展**——通过注册轮廓描述而非修改 switch
3. **效果可叠加，顺序可控**——用有序 Layer 列表替代硬编码
4. **几何可组合**——支持复合轮廓、布尔运算、截面拉伸
5. **向后兼容**——现有 DSL 无需改动，新能力通过可选字段渐进开放

---

## 三、核心概念拆解

### 3.1 三层分离

```
┌─────────────────────────────────────────┐
│  Layer 3: Effect Pipeline（效果管线）     │  ← 与形状无关的视觉层
│  blur / glow / grain / inner-shadow      │
├─────────────────────────────────────────┤
│  Layer 2: Surface（表面材质）             │  ← per-face 的填充描述
│  fill / stroke / texture / gradient      │
├─────────────────────────────────────────┤
│  Layer 1: Geometry（几何轮廓）            │  ← 纯数学，不含颜色
│  面的多边形集合 + 法线方向 + Z 层序       │
└─────────────────────────────────────────┘
```

三层之间通过定义好的接口通信，任意一层可独立替换。

### 3.2 Geometry：面描述

一个 3D 形状在等轴测投影下被分解为若干**具名的面（Face）**，每个面携带：

```go
type Face struct {
    Name     string      // "top" | "left" | "right" | 自定义名
    Points   [][2]float64 // 投影后的屏幕坐标多边形（顺时针）
    Normal   [3]float64  // 世界空间法线，用于光照和可见性测试
    ZOrder   int         // painter 顺序，越小越先画
    IsVisible bool       // 背面剔除结果
    // 世界空间坐标，供 effect 层计算实际尺寸
    WorldBBox [2][3]float64 // [min, max] in world coords
}
```

形状的职责收窄为：**给定 width/depth/height，产出一个 Face 列表**。

### 3.3 ShapeProvider：形状注册表

```go
type ShapeProvider interface {
    // Name 返回这个 provider 处理的 shape 名称列表（含别名）
    Names() []string
    // Faces 计算该形状在给定尺寸下的面集合
    Faces(width, depth, height float64, params map[string]any) []Face
    // ContentAnchor 返回内容（图标 + 标签）应放置的面的 Name
    ContentAnchor() string
    // Footprint 返回形状的世界坐标占地矩形，供 layout solver 使用
    Footprint(width, depth, height float64) (w, d float64)
}

// 全局注册表
var shapeRegistry = map[string]ShapeProvider{}

func RegisterShape(p ShapeProvider) {
    for _, name := range p.Names() {
        shapeRegistry[name] = p
    }
}
```

内置形状（box、cylinder、cloud 等）以 `init()` 注册。外部包可在导入时注册自定义形状，不需要 fork 核心代码。

### 3.4 Surface：表面材质

每个 Face 关联一个 `Surface`，描述如何填充和描边：

```go
type Surface struct {
    Fill   FillSpec    // 见下
    Stroke StrokeSpec
}

type FillSpec struct {
    Kind     FillKind  // Solid | LinearGradient | RadialGradient | Pattern | Texture | None
    Color    string    // Solid 时使用
    Gradient *GradientSpec
    Pattern  *PatternSpec
    Texture  *TextureSpec  // 新增：支持图片/SVG 纹理
}

type GradientSpec struct {
    Kind   GradientKind // Linear | Radial | Angular
    Stops  []ColorStop  // 任意数量色标，不再限于 from/to
    Dir    string       // 任意角度，不再限于 5 方向
    // 当 Kind=Linear: 角度 (度)
    // 当 Kind=Radial: 中心点 (cx, cy) 相对于面局部坐标 0..1
    Angle  float64
    Cx, Cy float64
}

type ColorStop struct {
    Offset float64 // 0..1
    Color  string
}

type PatternSpec struct {
    Kind      PatternKind // Hatch | Dots | Grid | Checkerboard | Custom
    Color     string
    Spacing   float64
    Angle     float64
    // 新增：透视校正，将 pattern 映射到等轴测面坐标系
    // true = pattern 跟随面的等轴测变换（看起来"贴"在面上）
    Projected bool
}

type TextureSpec struct {
    URI       string  // data:image/... 或 https://...
    Projected bool    // 同上，透视映射
    Opacity   float64
    BlendMode string  // normal | multiply | screen | overlay
}

type StrokeSpec struct {
    Layers []StrokeLayer // 多重描边，按顺序叠加
}

type StrokeLayer struct {
    Color     string
    Width     float64
    Dash      string
    Offset    float64 // 正值=外描边，负值=内描边（通过 clip-path 实现）
    Opacity   float64
}
```

### 3.5 Effect Pipeline：效果管线

效果是附加在**形状整体**上的 SVG filter/overlay 层，独立于 Surface：

```go
type Effect interface {
    // EmitDef 向 <defs> 写入必要的 filter/pattern 定义，返回引用 id
    EmitDef(defs *strings.Builder, id string, bbox ScreenBBox) string
    // Apply 将效果应用到已渲染的形状 SVG 片段上
    // 返回包裹后的 SVG 字符串
    Apply(inner string, defID string, bbox ScreenBBox) string
    // ZOffset 效果的 Z 层偏移：负值=在形状下方，正值=在形状上方
    ZOffset() int
}
```

内置 Effect 实现：

| Effect | 描述 | 当前状态 |
|--------|------|---------|
| `DropShadowEffect` | 投影阴影，支持多重叠加 | 已有，单层 |
| `BackglowEffect` | 形状感知的外发光（用形状轮廓而非固定圆） | 已有，固定圆 |
| `InnerGlowEffect` | 内发光 | **新增** |
| `GrainEffect` | 胶片噪声，支持 per-face | 已有，仅全局 |
| `BlurEffect` | 高斯模糊，用于 ghost/fog 节点 | **新增** |
| `OutlineEffect` | 轮廓高光描边（区别于 stroke） | **新增** |
| `LightingEffect` | 模拟方向光源，自动调整三面明暗 | **新增** |

Effect 列表是有序的，用户通过 DSL 的 `effects` 数组控制顺序：

```yaml
style:
  effects:
    - kind: backglow
      color: "#A78BFA"
      radius: 46
    - kind: grain
      intensity: 0.3
    - kind: innerGlow      # 新能力
      color: "#FFFFFF"
      radius: 8
      opacity: 0.4
```

### 3.6 几何组合：CompoundShape

支持将多个基础形状组合为复合轮廓：

```go
type CompoundShape struct {
    // 子形状列表，每个子形状有局部偏移
    Parts []ShapePart
    // 布尔运算：Union | Subtract | Intersect
    // 当前阶段仅实现 Union（painter order 叠加）
    // 后续阶段实现基于 clip-path 的 Subtract
    Op BooleanOp
}

type ShapePart struct {
    Provider ShapeProvider
    Width, Depth, Height float64
    // 局部偏移（世界坐标）
    OffX, OffY, OffZ float64
    // 这个子形状的 Surface 覆盖（nil = 继承父级）
    SurfaceOverride *SurfaceMap
}
```

这使得可以描述：
- **带顶盖的容器**：底部 box + 顶部薄板（两个 rectangle 的 Union）
- **缺口形状**：Subtract 实现穿孔平台
- **嵌套几何**：服务器机架（多个 box 叠加，各自独立材质）

---

## 四、新渲染管线

### 4.1 整体流程

```
DSL
  ↓
StyleResolver（不变）
  ↓
GeomResolver                  ← 新：shape name → ShapeProvider
  ↓  
ShapeProvider.Faces()         ← 产出 []Face（纯几何，无颜色）
  ↓
SurfaceResolver               ← 新：为每个 Face 分配 Surface
  ↓
FaceRenderer                  ← 新：Face + Surface → SVG 片段
  ↓
ContentRenderer               ← 不变：icon + label 投影到 ContentAnchor 面
  ↓
EffectPipeline.Apply()        ← 新：有序 Effect 列表逐层应用
  ↓
SVG string
```

### 4.2 SurfaceMap：面到材质的映射

```go
// SurfaceMap 描述一个形状的完整表面材质。
// 键是 Face.Name，"*" 是通配（匹配所有未显式指定的面）。
type SurfaceMap map[string]Surface

// 示例：顶面用渐变，侧面用纯色，所有面共享同一描边
SurfaceMap{
    "top":   Surface{Fill: FillSpec{Kind: LinearGradient, Gradient: &GradientSpec{...}}},
    "left":  Surface{Fill: FillSpec{Kind: Solid, Color: "#3A6FBA"}},
    "right": Surface{Fill: FillSpec{Kind: Solid, Color: "#5589D6"}},
    "*":     Surface{Stroke: StrokeSpec{Layers: []StrokeLayer{{Color: "#1D3A66", Width: 1.5}}}},
}
```

DSL 侧对应新增的 `style.faces` 字段：

```yaml
style:
  faces:
    top:
      fill:
        kind: linearGradient
        stops:
          - { offset: 0, color: "#A78BFA" }
          - { offset: 1, color: "#7C5CFC" }
        angle: 135
    left:
      fill: { kind: solid, color: "#3730A3" }
    right:
      fill: { kind: solid, color: "#4338CA" }
```

### 4.3 FaceRenderer 实现

```go
func renderFace(sb *strings.Builder, f Face, s Surface, defs *strings.Builder) {
    fillRef := resolveFill(defs, f.Name, s.Fill)
    
    // 主多边形
    writePolygon(sb, f.Name, f.Points, fillRef)
    
    // 多重描边（从外到内叠加）
    for i, layer := range s.Stroke.Layers {
        renderStrokeLayer(sb, defs, f, layer, i)
    }
}

func resolveFill(defs *strings.Builder, faceID string, spec FillSpec) string {
    switch spec.Kind {
    case Solid:
        return spec.Color
    case LinearGradient:
        id := "grad-" + faceID
        emitLinearGradientV2(defs, id, spec.Gradient)
        return "url(#" + id + ")"
    case RadialGradient:
        id := "rgrad-" + faceID
        emitRadialGradient(defs, id, spec.Gradient)
        return "url(#" + id + ")"
    case Pattern:
        id := "pat-" + faceID
        emitPatternV2(defs, id, spec.Pattern, f.Points) // 透视映射
        return "url(#" + id + ")"
    case Texture:
        id := "tex-" + faceID
        emitTextureClip(defs, id, spec.Texture, f.Points)
        return "url(#" + id + ")"
    case None:
        return "none"
    }
}
```

### 4.4 透视纹理映射

Pattern/Texture 的 `Projected: true` 模式下，利用 SVG `patternTransform` 将图案变换到等轴测面坐标系：

```
等轴测顶面变换矩阵（已知）：
matrix(cos30, sin30, -cos30, sin30, originX, originY)

将此矩阵应用到 patternTransform，使图案随面投影，
而非保持屏幕空间对齐。
```

视觉效果：网格纹理"贴"在盒子顶面上，随视角倾斜，而非保持水平。

---

## 五、内置形状的重新设计

### 5.1 现有形状迁移路径

所有现有形状迁移到新接口，同时保持行为不变（golden test 不退化）：

```
RenderIsoBox      → BoxShapeProvider      (rectangle / square / default)
RenderIsoCylinder → CylinderShapeProvider
RenderIsoCloud    → CloudShapeProvider
RenderIsoPerson   → PersonShapeProvider
RenderIsoSphere   → SphereShapeProvider
RenderIsoText     → TextShapeProvider
```

迁移策略：新 provider 内部可以继续调用老渲染函数，只需将输出包装成符合新接口的 Face 列表。这样可以分阶段迁移，不需要一次性重写所有几何。

### 5.2 架构演进后可支持的几何构型分类

当前架构的几何宇宙只有 6 种手写形状（box / cylinder / sphere / cloud / person / text 板），其余 d2 形状全部塌缩为长方体。新架构（ShapeProvider + 通用拉伸 + CompoundShape）落地后，可支持的几何构型按**生成方式**分为六大类。每类对应一种生成器，类内的具体形状只是参数差异，边际成本接近零。

#### 类别 A：直棱柱族（任意底面 × 垂直拉伸）

生成器：`PrismShapeProvider`。给定底面多边形 + 高度，自动完成等轴测投影、侧面可见性剔除、Face 排序。

| 形状 | 底面 | 典型语义（IT 基础设施）|
|------|------|------|
| `diamond` | 旋转 45° 正方形 | 决策节点、路由判断 |
| `hexprism` | 正六边形 | API 网关、中间件 |
| `triprism` | 三角形 | 告警、单向分发 |
| `parallelogram` | 平行四边形 | 数据 I/O、ETL 节点 |
| `octprism` | 正八边形 | 防火墙（八角=stop 语义）|
| `star` | 星形多边形 | 高亮/特殊节点 |

收益最大的一类：云架构图缺失几何的约 70% 属于此类。

#### 类别 B：异形轮廓拉伸族（任意 SVG path × 垂直拉伸）

生成器：`CustomPathShapeProvider`。底面是任意贝塞尔/折线轮廓，采样为点集后做与类别 A 相同的拉伸管线。cloud 形状现有的逐段可见性算法（`shape_cloud.go` 的 `dy > dx` 测试）直接泛化复用。

| 形状 | 轮廓来源 | 典型语义 |
|------|------|------|
| `shield` | 盾形贝塞尔轮廓 | 安全域、WAF |
| `arrow-slab` | 箭头形板 | 流程方向、流量指向 |
| `gear-slab` | 齿轮轮廓 | 配置中心、调度器 |
| `notched-box` | 带卡扣矩形 | 消息队列（真正的队列形状，非 cylinder 别名）|
| `custom_path` | 用户提供 path | 任意品牌/领域专属形状 |

#### 类别 C：旋转体族（母线绕 z 轴旋转）

生成器：`RevolveShapeProvider`。给定侧面母线（半径随高度变化的函数），按等轴测椭圆截面逐层生成。cylinder 和 sphere 是其特例（母线分别为竖直线和半圆）。

| 形状 | 母线 | 典型语义 |
|------|------|------|
| `frustum` | 斜直线（圆台）| 对象存储桶（S3 类）、漏斗 |
| `dome` | 四分之一圆弧 | 安全罩、隔离区（半透明壳体）|
| `cone` | 收敛到点的直线 | 流量汇聚、负载均衡分发 |
| `torus` | 偏离轴线的圆 | 缓存环、一致性哈希环 |
| `capsule` | 直线+两端圆弧 | Pod、容器实例 |

#### 类别 D：斜切/变截面族（顶底面不平行或不相似）

生成器：棱柱生成器的扩展参数（`TopInset` / `TopTilt` / `TopOffset`）。顶面相对底面可缩放、倾斜、平移，侧面自动变为梯形。

| 形状 | 变形参数 | 典型语义 |
|------|------|------|
| `wedge` | 顶面单侧压低 | 坡道、层级过渡、流量爬升 |
| `pyramid` / `ziggurat` | 顶面内缩（到点/到小矩形）| 分层架构、数据金字塔 |
| `chamfered-box` | 顶面均匀内缩 | 设备机箱（带斜边的硬件质感）|

#### 类别 E：复合结构族（多部件组合 + 布尔运算）

生成器：`CompoundShape`（Union 为主，Subtract 基于 clip-path）。部件间有固定的结构关系，整体作为一个形状对外暴露。

| 形状 | 组合方式 | 典型语义 |
|------|------|------|
| `rack` | 竖直框架（4 柱+顶底板）+ 内部可插入 slab 槽位 | 服务器机架——数据中心图核心几何 |
| `chip` | 扁方体 + 底面管脚阵列 | GPU/TPU/ASIC 芯片 |
| `tube` / `pipe` | 圆柱 Subtract 内圆柱 | 数据管道、专线、VPN 隧道 |
| `tray` | 薄板 + 四边凸起沿 | 设备托盘、承载平台 |
| `holed-platform` | 板 Subtract 任意轮廓 | 镂空平台、井口 |

person 现有实现（半球躯干+球头）本质就是一个硬编码的 CompoundShape，迁移后成为该族的内置实例。

#### 类别 F：薄板/边界族（二维语义优先的扁平几何）

生成器：棱柱生成器的低高度特化 + 边界专属 Surface 语义。这一类的关键不是几何而是**边界表达**——支持虚线边界、仅边框无填充、嵌套层级。

| 形状 | 形态 | 典型语义 |
|------|------|------|
| `plane` | 大面积薄板，支持虚线边界 | VPC / 子网 / 可用区 / Region 的嵌套分层 |
| `boundary` | 仅边框线的扁平区域（无底板填充）| C4 系统边界、信任域 |
| `zone-shell` | 半透明竖直围栏 | 网络隔离区的立体"围住"语义 |

当前 group 只能做实心底板，无法表达"虚线嵌套边界"这一云架构图最高频的容器语义；此类构型是 group 的语义升级。

#### 类别 G：阵列族（基元 × 三维重复）

生成器：`ArrayShapeProvider`。给定一个基元形状（任意 A-D 类）+ 三轴重复参数，按等轴测 painter 顺序（后排先画）批量实例化。本质是 CompoundShape 的特化，但单独成类的理由是：**重复语义在 DSL 层一等公民化**——逐个手写 N×M×K 个 part 既不可写也不可 diff，必须由参数生成。

```yaml
shape: array3d
geom: { w: 200, d: 200, h: 120 }
params:
  cell: { shape: rectangle, w: 18, d: 18, h: 18 }
  count: { x: 8, y: 8, z: 4 }      # 8×8×4 小方块阵列
  gap: 4
  fade: back                       # 可选：后排透明度衰减，强化体积感
```

| 形状 | 阵列形态 | 典型语义（AI / 大数据）|
|------|------|------|
| `array3d` | N×M×K 立体网格 | 张量、embedding 矩阵、GPU 集群 |
| `array2d` | N×M 平面网格 | 分区表、shard 矩阵、节点池 |
| `array1d` | 单轴线性排列 | 流水线 stage、副本组（现有 stack 的泛化）|

现有 `stack` 原语（垂直复制）成为 `array1d` 沿 z 轴的特例，迁移后保持 DSL 兼容。

#### 类别 H：竖立面板族（朝向观察者的直立薄板）

生成器：棱柱生成器新增 `orient: upright` 参数。现有所有几何默认"底面贴地、顶面朝上"，而 App 开发图的核心节点——**手机屏幕、浏览器窗口、仪表盘**——需要竖立面板：薄板立在地面上，主展示面朝向观察者（等轴测的左前或右前面）。

技术差异：内容（icon/label/截图纹理）不再投影到 top face，而是投影到竖立面。投影矩阵从 top 面的 `matrix(cos30, sin30, -cos30, sin30, ...)` 换为左/右立面的剪切矩阵，`ContentAnchor()` 接口返回对应立面名即可，渲染管线无需特判。

| 形状 | 形态 | 典型语义（App 开发）|
|------|------|------|
| `screen` | 竖立圆角薄板，正面可贴纹理 | 手机 App 界面、UI 原型 |
| `browser-panel` | 竖立薄板 + 顶部标签条 | Web 前端、管理后台 |
| `billboard` | 竖立板 + 支撑脚 | 仪表盘、监控大屏 |
| `card` | 微倾斜的竖立卡片 | 文档、凭证、消息卡片 |

#### 覆盖率对照

| 需求域 | 现架构 | 新架构 |
|------|------|------|
| 流程/决策形状（diamond、hexagon 等）| 全部塌缩为盒子 | 类别 A 全覆盖 |
| 网络边界（VPC/子网嵌套）| 实心底板凑合 | 类别 F |
| 物理基础设施（机架/芯片/刀片）| 不可表达 | 类别 E |
| 存储语义（桶/环）| 仅圆柱 | 类别 C |
| 流量隐喻（漏斗/坡道/汇聚）| 不可表达 | 类别 C + D |
| 品牌/领域专属异形 | 不可表达 | 类别 B |
| 张量/集群网格（AI）| 不可表达 | 类别 G |
| 屏幕/UI 面板（App 开发）| 不可表达 | 类别 H |

八类生成器中，A、B、D 共享同一条"轮廓拉伸"管线（一次投影数学，三类收益），C 是独立的旋转体管线，E 是组合器（依赖前四类作为部件），F、H 是棱柱生成器的语义/朝向特化，G 是 E 的重复特化。实施顺序上 **A → B/D → F/H → C → G → E** 收益递减、成本递增。

#### 明确不在本方案范围内的需求（Roadmap 记录）

以下两类需求在覆盖率评估中被识别，但它们不是**节点几何**问题，归属其他子系统，本方案不解决：

1. **带体积的管道连接器**——大数据 DAG 图中"数据流量体积随管道粗细变化"的表达。这是 connector 子系统从"线条"升级为"3D 截面拉伸体"的工程，依赖本方案的 C 类旋转体管线作为基础，但路由、避障、与节点的接合都是 connector 层的事。
2. **场景级区域 overlay**——跨多个节点的高亮域/着色区（如"这条调用链上的 5 个节点同属一次故障域"）。这是渲染层在 inject 阶段的新图层，与单节点几何无关。

这两项与第七、八类（G/H）的区别在于：G/H 是纯节点几何，加进生成器体系即可；上述两项动的是其他子系统的架构，混入本方案会导致范围失控。

### 5.3 通用棱柱（n-gon prism）

box 和 hexagon 都是棱柱的特例。抽象出通用棱柱 provider：

```go
type PrismShapeProvider struct {
    Sides int     // 3=三棱柱, 4=长方体, 6=六棱柱, ...
    Inset float64 // 顶面内缩比例（0=直角柱，>0=梯形柱）
}

func (p *PrismShapeProvider) Faces(w, d, h float64, _ map[string]any) []Face {
    // 1. 生成正 n 边形顶点（局部坐标 0..W, 0..D）
    // 2. 投影到等轴测屏幕坐标（z=0 底面，z=h 顶面）
    // 3. 可见性测试（法线朝向观察者的面）
    // 4. 返回 top + n 个侧面，侧面按 ZOrder 排序
}
```

DSL：

```yaml
shape: hexprism
geom: { w: 120, d: 120, h: 80 }
```

### 5.4 自定义轮廓拉伸（CustomPathShape）

允许用户提供 SVG path 作为底面轮廓，系统自动做等轴测拉伸：

```go
type CustomPathShapeProvider struct{}

func (c *CustomPathShapeProvider) Names() []string { return []string{"custom_path"} }

func (c *CustomPathShapeProvider) Faces(w, d, h float64, params map[string]any) []Face {
    pathData := params["path"].(string)       // SVG path d= 属性
    outline  := sampleSVGPath(pathData, 64)   // 采样为折线点集
    // 归一化到 [0,W] x [0,D]
    outline = normalizeOutline(outline, w, d)
    // 等轴测拉伸
    return extrudeOutline(outline, h)
}

func extrudeOutline(outline [][2]float64, h float64) []Face {
    // 顶面：outline 投影到 z=h
    // 侧面：逐段分析可见性，生成侧壁多边形
    // 底面：outline 投影到 z=0（通常不可见）
}
```

DSL：

```yaml
shape: custom_path
geom: { w: 160, d: 100, h: 40 }
params:
  path: "M 0,50 Q 50,0 100,50 Q 50,100 0,50 Z"   # 任意 SVG 路径
```

---

## 六、DSL 变更（向后兼容）

### 6.1 新增字段，全部可选

现有 DSL 字段**一律保留**，新字段作为可选扩展：

```yaml
nodes:
  scene:
    shape: composite
    parts:
      - id: hero
        shape: rectangle       # 旧字段，继续有效
        geom: { w: 160, d: 160, h: 48 }
        
        # 旧样式字段（继续有效）
        style:
          palette:
            top: "#7C5CFC"
            left: "#4338CA"
            right: "#5B21B6"
        
        # 新字段：per-face 材质（优先级高于 palette）
        style:
          faces:
            top:
              fill:
                kind: linearGradient
                stops:
                  - { offset: 0,   color: "#A78BFA" }
                  - { offset: 0.5, color: "#7C5CFC" }
                  - { offset: 1,   color: "#6D28D9" }
                angle: 150
              stroke:
                layers:
                  - { color: "#C4B5FD", width: 2, offset: -1 }  # 内高光
                  - { color: "#4C1D95", width: 1.5 }             # 外描边
            left:
              fill:
                kind: texture
                uri: "iso://pattern/carbon"
                projected: true
                blendMode: multiply
                opacity: 0.3
          
          # 新字段：有序 effect 列表
          effects:
            - kind: backglow
              color: "#A78BFA"
              radius: 46
              shapeAware: true       # 新参数：沿形状轮廓而非固定圆
            - kind: innerGlow
              color: "#FFFFFF"
              radius: 8
              opacity: 0.3
            - kind: grain
              intensity: 0.25
              perFace: true          # 新参数：各面独立噪声
        
        # 新字段：自定义形状
        # shape: custom_path
        # params:
        #   path: "M 0,0 L 100,0 L 80,100 L 20,100 Z"
```

### 6.2 优先级规则

```
style.faces[faceName].fill   >  style.palette.top/left/right
style.faces["*"].fill        >  style.palette（通配）
style.effects（数组）        >  style.effects.{backglow,dropShadow,...}（旧字段）
```

旧字段在 StyleResolver 中被转换为新格式，渲染器只看新格式，完全向后兼容。

---

## 七、实施计划

### Phase 1：基础设施（不改变任何现有输出）

1. 定义 `Face`、`ShapeProvider`、`Surface`、`SurfaceMap` 接口和类型
2. 实现 `shapeRegistry` 注册表
3. 将 `BoxShapeProvider` 作为第一个迁移对象，输出与现有 `RenderIsoBox` 完全一致（golden test 全绿）
4. 实现 `FaceRenderer` + `resolveFill`，替换 `writeFace` 调用

**验收**：`go test ./...` 全绿，无 golden file 变化。

### Phase 2：Surface 扩展

5. 实现多色标线性渐变（`GradientSpec` with `Stops`）
6. 实现径向渐变
7. 实现多重描边（`StrokeSpec.Layers`）
8. 实现投影 Pattern（`Projected: true`）
9. 新增 DSL `style.faces` 字段 + 对应 validator

**验收**：新 DSL 能产出新视觉效果；旧 DSL 输出不变。

### Phase 3：Effect Pipeline

10. 定义 `Effect` 接口
11. 将现有 backglow / grain / dropShadow 迁移为实现此接口的类型
12. 实现有序 `EffectPipeline`，替换 `emitBoxDefs` 中的硬编码顺序
13. 新增 `InnerGlowEffect`、`BlurEffect`、`OutlineEffect`
14. 实现形状感知的 backglow（沿真实轮廓而非固定圆模糊）

### Phase 4：形状扩展

15. 完成所有现有形状向 `ShapeProvider` 的迁移
16. 实现 `PrismShapeProvider`（通用 n 棱柱）
17. 实现 `CustomPathShapeProvider`（SVG path 拉伸）
18. 新增 `diamond`、`wedge`、`arch` 等内置形状

### Phase 5：CompoundShape（可选，后期）

19. 实现 `CompoundShape` Union 操作（painter order 叠加）
20. 实现基于 `clip-path` 的 Subtract 操作

---

## 八、关键设计决策

### 决策一：为什么不改用 SVG DOM 树而继续用字符串

SVG DOM 树（`encoding/xml` 或自定义树）会引入大量 allocation，且现有 inject.go 的字符串注入模式已经稳定。维持字符串输出，但在**形状渲染层**内部改用结构化中间表示，只在最后一步序列化为字符串。这样改动范围最小，inject.go 无需变动。

### 决策二：Effect 接口 vs 纯函数

选择接口而非纯函数，原因是部分 Effect（如 grain）需要在 `<defs>` 和实际节点引用两处写入，接口的 `EmitDef` / `Apply` 两步模型能清晰表达这个分离。

### 决策三：`style.faces` vs 扩展 `style.palette`

选择新的 `faces` 字段而非扩展 `palette`，原因是 `palette` 的语义是"三面颜色"，语义边界清晰。把渐变/纹理/多重描边塞入 palette 会使语义混乱。新字段保持旧字段语义完整，在 resolver 层合并。

### 决策四：透视纹理映射的精度边界

SVG `patternTransform` 只支持仿射变换（矩阵），等轴测投影恰好是仿射变换（无透视畸变），因此可以精确映射。三维透视（perspective projection）超出 SVG 原生能力，不在本方案范围内。

---

## 九、总结

本方案的本质是：将当前**"形状 = 参数化的渲染函数"**的封闭模型，重构为**"形状 = 几何描述 + 独立材质 + 可组合效果管线"**的开放模型。

核心收益：
- **任意形状 × 任意效果**：消除"哪个形状支持哪个效果"的不对等
- **效果可叠加**：多重阴影、多重描边、发光 + 噪声同时存在
- **形状可扩展**：第三方可注册新形状，不需要 fork
- **渐进迁移**：四个 Phase 每个都可独立交付，全程 golden test 守护
