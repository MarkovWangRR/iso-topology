package isotopo

// Studio detail-editor localization. The field metadata in fields.go is authored
// in English; these tables translate the visible copy (group/section names,
// labels, one-line descriptions) for the UI-language toggle. Keying by the
// English STRING — not the field path — keeps it unambiguous when one path
// reuses a label across contexts (e.g. style.palette.top is "Fill color" in the
// simple Fill group and "Top" in the per-face group), and any string without an
// entry falls through to English, so partial coverage degrades gracefully.

var fieldGroupZH = map[string]string{
	"General":    "常规",
	"Content":    "内容",
	"ShapeSize":  "形状与大小",
	"IconText":   "图标与文字",
	"Appearance": "外观",
	"Arrange":    "排布",
	"Size":       "尺寸",
	"Position":   "位置",
	"Layout":     "布局",
	"Border":     "边框",
	"Fill":       "填充",
	"Gradients":  "渐变",
	"Label":      "文字",
	"Effects":    "阴影和纹理",
	"Background": "背景",
	"Connection": "连接",
	"Style":      "样式",
	"Routing":    "走线",
	"Line":       "线条",
}

// Second-level card titles (Field.Sub).
var fieldSubZH = map[string]string{
	"Size":               "尺寸",
	"Label":              "标签",
	"Icon":               "图标",
	"Typography":         "文字",
	"Fill":               "填充",
	"Endpoints":          "端点",
	"Routing":            "走线",
	"Text":               "文字",
	"Stroke":             "描边",
	"Gradient":           "渐变",
	"Top face":           "顶面",
	"Left face":          "左面",
	"Right face":         "右面",
	"Border":             "边框",
	"Shape detail":       "形态细化",
	"Light & blur":       "透明与模糊",
	"Drop shadow":        "投影",
	"Glow":               "辉光",
	"Texture":            "纹理",
	"Relative placement": "相对放置",
	"Precise offset":     "精确偏移",
	"Repeat":             "重复",
	"Container layout":   "容器布局",
}

var fieldLabelZH = map[string]string{
	"Base":                     "底色",
	"Direction":                "方向(可选)",
	"Label text":               "标签文字",
	"Placement":                "贴面方式",
	"Style":                    "样式",
	"Split left / right faces": "左右面分离着色",
	"X":                        "X",
	"Y":                        "Y",
	"Z lift":                   "Z 抬升",
	"Radius":                   "半径",
	"Spacing":                  "间距",
	"Angle":                    "角度",
	"Texture":                  "纹理",
	"Copies":                   "副本数",
	"Child gap":                "子项间距",
	"Label":         "标签",
	"Shape":         "形状",
	"Style preset":  "样式预设",
	"Icon":          "图标",
	"Icon color":    "图标颜色",
	"Width":         "宽度",
	"Depth":         "深度",
	"Height":        "高度",
	"Right of":      "右侧靠",
	"Left of":       "左侧靠",
	"Above":         "上方",
	"Behind":        "后方",
	"In front of":   "前方",
	"Place gap":     "放置间距",
	"Offset X":      "偏移 X",
	"Offset Y":      "偏移 Y",
	"Offset Z":      "偏移 Z",
	"Stack count":   "堆叠层数",
	"Stack gap":     "堆叠间距",
	"Mode":          "模式",
	"Gap":           "间距",
	"Columns":       "列数",
	"Padding":       "内边距",
	"Align":         "对齐",
	"Border color":  "边框颜色",
	"Border width":  "边框宽度",
	"Border style":  "边框样式",
	"Fill color":    "填充色",
	"Color":         "颜色",
	"Dash":          "虚线",
	"Top":           "顶面",
	"Left":          "左面",
	"Right":         "右面",
	"Top from":      "顶面 起始",
	"Top to":        "顶面 终止",
	"Top dir":       "顶面 方向",
	"Left from":     "左面 起始",
	"Left to":       "左面 终止",
	"Left dir":      "左面 方向",
	"Right from":    "右面 起始",
	"Right to":      "右面 终止",
	"Right dir":     "右面 方向",
	"Top grad to":   "顶面 渐变到",
	"Left grad to":  "左面 渐变到",
	"Right grad to": "右面 渐变到",
	"Split faces":   "拆分面",
	"Gradient from": "渐变 起点",
	"Gradient to":   "渐变终点(可选)",
	"Text color":    "文字颜色",
	"Font size":     "字号",
	"Weight":        "字重",
	"Font family":   "字体",
	"Orientation":   "朝向",
	"Corner radius": "圆角半径",
	"Opacity":       "不透明度",
	"Blur":          "模糊",
	"Shadow color":  "阴影颜色",
	"Shadow dx":     "阴影 dx",
	"Shadow dy":     "阴影 dy",
	"Shadow blur":   "阴影模糊",
	"Glow color":    "辉光颜色",
	"Glow radius":   "辉光半径",
	"Glow opacity":  "辉光不透明度",
	"Pattern":       "纹理",
	"Pattern color": "纹理颜色",
	"Pattern spacing": "纹理间距",
	"Pattern angle": "纹理角度",
	"Grid pattern":  "网格图案",
	"Grid color":    "网格颜色",
	"Grid step":     "网格步长",
	"From":          "起点(源)",
	"To":            "终点(目标)",
	"Routing":       "走线方式",
	"Arrowhead":     "箭头",
	"Elbow bias":    "折线方向",
	"Text":          "文字",
	"Background":    "背景色",
}

var fieldDescZH = map[string]string{
	"Text rendered on the node":                       "显示在节点上的文字",
	"Geometric form of the node":                      "节点的几何形状",
	"Named style from theme.presets":                  "来自 theme.presets 的命名样式",
	"iso://… ref, image URL, or pick a local file":    "iso://… 引用、图片 URL,或选择本地文件",
	"Tint for iso:// glyph/logo icons (blank = default ink)": "iso:// 图标的着色(留空 = 默认色)",
	"Sibling id to sit to the right of":               "靠其右侧放置的同级节点 id",
	"Gap in cells":                                    "间距(单位:格)",
	"World-space fine-tune":                           "世界坐标微调",
	"Lift (the axis the solver never sets)":           "抬升(求解器从不设置的轴)",
	"Replica layers (1 = none)":                       "副本层数(1 = 无)",
	"Z-step between replicas":                         "副本之间的 Z 向间距",
	"Auto-arrange children":                           "自动排布子节点",
	"Spacing between children, cells":                 "子节点间距(格)",
	"Grid mode only":                                  "仅网格模式",
	"Inner margin, cells":                             "内边距(格)",
	"Cross-axis alignment":                            "交叉轴对齐",
	"Outline color (CSS color)":                       "描边颜色(CSS 颜色)",
	"Solid, dashed, or dotted":                        "实线、虚线或点线",
	"Surface fill (CSS color)":                        "表面填充(CSS 颜色)",
	"Caption color (CSS color)":                       "文字颜色(CSS 颜色)",
	"Caption size in px":                              "文字字号(px)",
	"Font weight":                                     "字体粗细",
	"CSS font stack":                                  "CSS 字体栈",
	"iso (on-face) or screen (flat below)":            "iso(贴面)或 screen(下方平铺)",
	"Rounds vertical edges":                           "圆角化竖直棱边",
	"Base color (also gradient start)":                "底色(也是渐变起点)",
	"Gradient end; blank = solid":                     "渐变终点;留空 = 纯色",
	"Shade left/right faces independently (needs corner radius)": "左右面独立着色(需圆角半径)",
	"Source-end color; set both ends for a line gradient":        "源端颜色;两端都填即线性渐变",
	"Target-end color":                                "目标端颜色",
	"0–1 whole-part transparency":                     "0–1 整体透明度",
	"Gaussian blur px — fog/ghost":                    "高斯模糊 px —— 雾化/虚化",
	"Drop shadow under the silhouette":                "轮廓下方的投影",
	"Soft halo behind the part":                       "部件背后的柔和光晕",
	"Top-face texture":                                "顶面纹理",
	"Degrees (hatch)":                                 "角度(斜线纹理)",
	"Canvas fill behind the diagram (CSS color)":      "图表背后的画布填充(CSS 颜色)",
	"Background texture":                              "背景纹理",
	"Grid/texture line color (CSS color)":             "网格/纹理线条颜色(CSS 颜色)",
	"Grid cell size in world units (blank = default)": "网格单元尺寸(世界单位,留空 = 默认)",
	"Outer breathing margin around the scene, px":     "场景外围留白(px)",
	"Source anchor — node id or node.face":            "源锚点 —— 节点 id 或 node.face",
	"Target anchor — node id or node.face":            "目标锚点 —— 节点 id 或 node.face",
	"Path style between endpoints":                    "端点之间的走线样式",
	"Marker at the target end":                        "目标端的箭头标记",
	"Orthogonal turn order":                           "正交折线的转向顺序",
	"Stroke color (CSS color)":                        "描边颜色(CSS 颜色)",
	"Text rendered mid-route":                         "显示在连线中部的文字",
}

// fieldGroupEN tidies camelCase group keys into friendly English titles, so the
// English UI never shows a raw key like "ShapeSize". Keys that are already
// human-readable (Appearance, Arrange) need no entry.
var fieldGroupEN = map[string]string{
	"ShapeSize": "Shape & Size",
	"IconText":  "Icon & Text",
}

// LocalizeFields returns a copy of fs with the visible copy localized for the
// Studio UI language. lang=="zh" translates everything; otherwise the English
// group keys are tidied into friendly titles. The input is not mutated.
func LocalizeFields(fs []Field, lang string) []Field {
	if len(fs) == 0 {
		return fs
	}
	out := make([]Field, len(fs))
	for i, f := range fs { // f is a copy — safe to edit
		if lang != "zh" {
			if v, ok := fieldGroupEN[f.Group]; ok {
				f.Group = v
			}
			out[i] = f
			continue
		}
		if v, ok := fieldGroupZH[f.Group]; ok {
			f.Group = v
		}
		if v, ok := fieldLabelZH[f.Label]; ok {
			f.Label = v
		}
		if v, ok := fieldDescZH[f.Desc]; ok {
			f.Desc = v
		}
		if v, ok := fieldSubZH[f.Sub]; ok {
			f.Sub = v
		}
		out[i] = f
	}
	return out
}
