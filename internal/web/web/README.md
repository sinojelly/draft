# SyncGhost - Vanilla SPA 版本

这是 SyncGhost 的 Vanilla SPA (原生单页应用) 完整版本，具备商业级产品标准。

## 🎯 项目特点

### 架构特点
- **单页体验 (SPA)**：使用客户端路由实现页面切换，无需刷新，支持浏览器前进/后退
- **物理模块化拆分**：代码按功能模块拆分到独立文件
- **ES6 原生 import**：使用标准的 ES6 模块系统
- **无框架依赖**：纯原生 JavaScript + HTML + CSS 实现
- **HTML Template**：使用模板字符串生成 DOM

### 性能优化
- ✅ **增量日志渲染**：解决大量日志导致的性能问题，采用 DOM Append 而非全量重绘
- ✅ **节流更新机制**：高频数据更新采用节流，避免浏览器卡顿
- ✅ **平滑动画过渡**：progress 和卡片都有流畅的过渡效果
- ✅ **内存管理**：自动清理超过500条的日志，控制DOM节点数量

### UX 体验优化
- ✅ **全局 Toast 通知**：取代所有 alert()，提供非阻塞的优雅提示
- ✅ **Empty State**：任务和账号为空时显示精美的空状态引导
- ✅ **卡片 Hover 效果**：悬停时轻微上浮和阴影变化，提升交互感
- ✅ **初次引导流程**：完整的 Onboarding 体验，引导用户完成初始配置
- ✅ **模态框交互**：支持点击背景关闭、平滑进出动画
- ✅ **响应式设计**：适配不同屏幕尺寸

### 完整功能
- ✅ **仪表盘**：实时数据监控、传输进度、终端日志、手动触发同步
- ✅ **任务管理**：创建、编辑、删除、暂停/启动同步任务
- ✅ **账号管理**：网盘账号绑定（支持二维码扫码）、解绑
- ✅ **系统设置**：全局配置参数（Web服务、日志、性能）
- ✅ **首次引导**：欢迎页 → 添加账号 → 创建任务 → 完成配置

## 📁 目录结构

```
vanilla-spa/
├── index.html              # 入口 HTML 文件
├── README.md              # 项目说明文档
├── styles/
│   └── main.css           # 全局样式（包含所有优化）
└── js/
    ├── main.js            # 应用入口
    ├── utils/
    │   ├── toast.js       # Toast 通知系统
    │   └── EventBus.js    # 事件总线（发布订阅）
    └── modules/
        ├── App.js         # 主应用模块
        ├── components/    # 组件模块
        │   ├── Sidebar.js
        │   ├── AddTaskModal.js
        │   ├── AddAccountModal.js
        │   └── SuccessModal.js
        └── pages/         # 页面模块
            ├── Onboarding.js
            ├── Dashboard.js
            ├── Tasks.js
            ├── Accounts.js
            └── Settings.js
```

## 🚀 使用方法

### 本地开发

由于使用了 ES6 模块，需要通过 HTTP 服务器运行（不能直接打开 HTML 文件）：

```bash
# 使用 Python 3
python -m http.server 8000

# 使用 Node.js (http-server)
npx http-server

# 使用 PHP
php -S localhost:8000
```

然后在浏览器访问：`http://localhost:8000`

### 初次使用

1. 首次打开会自动进入欢迎页（Onboarding）
2. 点击"开始使用"进入账号绑定流程
3. 完成账号绑定后，可选择立即配置任务或稍后配置
4. 完成配置后自动跳转到仪表盘

### 手动触发引导

如果已经完成初次配置，可以点击仪表盘右上角的"初次进入"按钮重新进入引导流程。

### 部署

将整个 `vanilla-spa` 目录上传到 Web 服务器即可。确保服务器支持：
- 正确的 MIME 类型（.js 文件为 application/javascript）
- 允许跨域请求（如果需要）

## 🛠️ 技术栈

- **HTML5**：语义化标签、原生 `<progress>` 元素
- **CSS3**：
  - CSS 变量（主题色系统）
  - Grid & Flexbox 布局
  - 动画（slideIn, slideOut）
  - 过渡效果（hover, focus）
  - 响应式设计
- **JavaScript ES6+**：
  - ES6 模块 (import/export)
  - 类 (Class)
  - 箭头函数
  - 模板字符串
  - 解构赋值
  - Promises
  - LocalStorage
  - EventBus 模式

## 🎨 设计理念

- **极简 (Minimalist)**：界面干净，专注核心功能
- **现代 (Modern)**：参考 Windows 11 视觉设计语言
- **轻量 (Lightweight)**：零外部依赖，加载快速
- **原生 (Native)**：充分利用浏览器原生能力
- **高性能**：优化渲染，避免不必要的DOM操作
- **用户友好**：流畅动画，清晰反馈，优雅降级

## ⚡ 性能优化细节

### 1. 日志增量渲染
```javascript
// ❌ 旧方式：全量重绘（卡顿）
logContainer.innerHTML = logs.map(...).join('');

// ✅ 新方式：增量插入（流畅）
const logDiv = document.createElement('div');
logDiv.innerHTML = '...';
logContainer.appendChild(logDiv);
```

### 2. 数据更新节流
```javascript
let lastUpdate = Date.now();
if (now - lastUpdate < 500) return; // 最多每秒2次更新
```

### 3. 内存控制
```javascript
const MAX_LOGS = 500;
while (logContainer.children.length > MAX_LOGS) {
  logContainer.removeChild(logContainer.firstChild);
}
```

## 🎯 UX 优化细节

### 1. Toast 通知系统
- 非阻塞式提示
- 自动消失（3秒）
- 支持成功/错误类型
- 流畅的滑入/滑出动画

### 2. Empty State 设计
- 居中的图标和文案
- 引导性的操作按钮
- 友好的视觉反馈

### 3. 微交互
- 卡片悬停上浮效果
- 按钮点击涟漪效果
- 进度条平滑过渡
- 模态框淡入淡出

## 📝 代码规范

- 使用 ES6 Class 封装组件
- 每个组件包含 `render()` 和 `attachEventListeners()` 方法
- 使用 EventBus 实现组件间通信（解耦）
- 避免全局变量污染
- 函数命名清晰（handle*, show*, close*, update*）
- 保持单一职责原则

## 🔧 扩展开发

### 添加新页面
1. 在 `js/modules/pages/` 创建新页面类
2. 在 `App.js` 中注册页面
3. 在 `Sidebar.js` 中添加导航项

### 添加新组件
1. 在 `js/modules/components/` 创建组件类
2. 实现 `render()` 和 `attachEventListeners()` 方法
3. 在需要的地方 import 并使用

### 使用 EventBus
```javascript
import { EventBus } from './utils/EventBus.js';

// 发送事件
EventBus.emit('TASK_CREATED', taskData);

// 监听事件
EventBus.on('TASK_CREATED', (data) => {
  console.log('Task created:', data);
});
```

## 🐛 已知问题

- 浏览器必须支持 ES6 模块（现代浏览器均支持）
- 需要 HTTP 服务器运行（不能直接打开 HTML）
- LocalStorage 存储容量有限（约5-10MB）

## 📄 许可证

MIT License

## 🙏 致谢

感谢设计需求文档的指导和优化建议文档的深度分析，让这个项目达到了商业级产品标准。
