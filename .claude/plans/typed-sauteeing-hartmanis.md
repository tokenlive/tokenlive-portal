# Portal 前端开发计划

## 已完成的工作

### Phase 1: API 类型修复 ✅
- 更新 `types/api.ts` 以匹配后端响应结构
- 添加正确的嵌套类型（ModelPrice, ModelMetrics, WorkspaceBalanceResponse 等）
- 更新 ConsoleOverviewResponse 和 ActivationStepResponse 类型

### Phase 2: API 客户端更新 ✅
- 更新 `lib/api.ts` 以正确处理新的响应结构
- 修复 fetchModelDetail 以提取嵌套的 `data` 字段

### Phase 3: 页面和组件更新 ✅
- 更新首页 (page.tsx) 使用新的模型列表响应
- 更新模型卡片组件处理嵌套的价格和指标
- 更新模型目录组件
- 更新控制台 Dashboard 页面显示余额
- 更新 API Keys 页面，添加花费显示和更好的 UI
- 更新 Settings 页面，修复登出功能
- 更新 Console Layout，添加实际的登出功能

### Phase 4: 新页面添加 ✅
- 添加注册页面 (/register)
- 添加 Accept Terms 页面 (/accept-terms)
- 添加模型详情页面 (/models/[slug])
- 更新登录页面支持从注册页面跳转

## 剩余工作

### Phase 5: 测试和优化 ⏳
- 运行 TypeScript 编译检查
- 运行构建检查错误
- 添加缺失的环境变量示例
- 测试端到端流程

## 技术栈
- Next.js 16+
- React 19+
- TypeScript
- Tailwind CSS
- shadcn/ui 组件
