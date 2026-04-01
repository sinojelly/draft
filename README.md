# SyncGhost - Vanilla SPA 版本

这是 SyncGhost 的 Vanilla SPA (原生单页应用) 版本。

## 架构特点

- **单页体验 (SPA)**：使用客户端路由实现页面切换，无需刷新
- **物理模块化拆分**：代码按功能模块拆分到独立文件
- **ES6 原生 import**：使用标准的 ES6 模块系统
- **无框架依赖**：纯原生 JavaScript + HTML + CSS 实现
- **HTML Template**：使用模板字符串生成 DOM

## 目录结构

```
vanilla-spa/
├── index.html              # 入口 HTML 文件
├── styles/
│   └── main.css           # 全局样式
└── js/
    ├── main.js            # 应用入口
    └── modules/
        ├── App.js         # 主应用模块
        ├── components/    # 组件模块
        │   └── Sidebar.js
        └── pages/         # 页面模块
            ├── Dashboard.js
            ├── Tasks.js
            ├── Accounts.js
            └── Settings.js
```

## 使用方法

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

### 部署

将整个 `vanilla-spa` 目录上传到 Web 服务器即可。

## 技术栈

- **HTML5**：语义化标签、原生 `<progress>` 元素
- **CSS3**：CSS 变量、Grid、Flexbox、动画
- **JavaScript ES6+**：
  - ES6 模块 (import/export)
  - 类 (Class)
  - 箭头函数
  - 模板字符串
  - 解构赋值
  - Promise / async-await

## 功能特性

✅ 仪表盘：实时数据监控、传输进度、终端日志
✅ 任务管理：创建、编辑、删除同步任务
✅ 账号管理：网盘账号绑定、解绑
✅ 系统设置：全局配置参数
✅ 响应式设计：适配不同屏幕尺寸
✅ 实时更新：模拟数据实时变化
✅ Toast 通知：操作反馈提示

## 设计理念

- **极简 (Minimalist)**：界面干净，专注核心功能
- **现代 (Modern)**：参考 Windows 11 视觉设计
- **轻量 (Lightweight)**：无外部依赖，加载快速
- **原生 (Native)**：充分利用浏览器原生能力
