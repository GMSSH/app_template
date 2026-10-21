# 系统跑分前端

当前阶段只实现 Vue 3 前端和 mock 跑分流程，不包含 Go 后端和真实服务器采集。

## 运行

    npm install
    npm run dev

检查与构建：

    npm run typecheck
    npm run build

## 当前功能

- 综合性能分展示；
- 完整 mock 跑分流程：CPU、内存、磁盘、网络；
- 普通跑分和快速测试；
- 阶段进度与运行状态；
- CPU、内存、磁盘分数卡片；
- 近 7 天趋势图；
- 服务器信息卡片；
- 中英文切换；
- 亮色/暗色切换；
- 非全屏窗口自适应和 Naive UI NScrollbar；
- 综合分数面板使用 Inspira UI AuroraBackground 官方 Vue 源码的渐变、动画和径向遮罩实现；
- 开发模式不调用 window.$gm.init()。

## 后端接入边界

后端确认前，所有跑分结果来自 src/App.vue 中的 mock 数据。后续接入时只替换数据/API 层，不重写页面展示、主题、国际化和跑分状态组件。
