# TokenLive Portal

TokenLive 的前端管理门户和用户界面。

## 项目结构

```
tokenlive-portal/
├── backend/          # Go 后端 API
├── web/              # Next.js 前端
│   ├── src/
│   │   ├── app/       # App Router 页面
│   │   ├── components/# React 组件
│   │   ├── lib/       # 工具函数和 API 客户端
│   │   └── types/     # TypeScript 类型定义
│   └── public/       # 静态资源
└── docs/             # 文档
```

## 功能特性

### 用户端
- 用户注册和登录 (邮件验证码 / Google OAuth / GitHub OAuth)
- 模型浏览和搜索
- 模型详情查看
- 仪表板概览
- API Keys 管理
- 用量与请求日志查看
- 账单与充值请求
- 账户设置

### 界面设计
- 深色主题 (TokenLive amber 色调)
- 响应式设计
- 现代化 UI (使用 shadcn/ui 和 Tailwind CSS)
- 流动的节点动画元素

## 开发

### 前端开发

```bash
cd web

# 安装依赖
npm install

# 开发模式
npm run dev

# 构建
npm run build

# 启动生产服务器
npm start
```

### 后端开发

```bash
cd backend

# 运行
go run ./cmd/portal-api
```

## 环境变量

在 `web/.env` 中配置：

```env
NEXT_PUBLIC_API_URL=http://localhost:8080
```

## 页面路由

- `/` - 模型列表首页
- `/models/[slug]` - 模型详情
- `/login` - 登录
- `/register` - 注册
- `/accept-terms` - 接受服务条款
- `/console/dashboard` - 控制台仪表板
- `/console/api-keys` - API Keys 管理
- `/console/usage` - 用量与请求日志
- `/console/billing` - 账单与充值请求
- `/console/settings` - 账户设置

## 技术栈

- **前端**: Next.js 16+, React 19+, TypeScript
- **样式**: Tailwind CSS 4+, shadcn/ui
- **后端**: Go 1.24+
- **数据库**: 参见 backend 配置
- **缓存**: Redis (可选)
