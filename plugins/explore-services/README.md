# 探索服务聚合

Mopie 的多平台影视探索插件。当前 `0.3.2-beta` 支持六个公开探索来源，后续按开发文档逐步加入媒体识别和订阅动作。

## 当前能力

- 一个独立的“探索”页面和主导航入口
- Bilibili、腾讯视频、CCTV、芒果TV、咪咕视频和 Bangumi 来源切换
- 各来源的电视剧、电影、综艺、纪录片、动漫或每日放送目录
- 分类切换、评分排序、地区/状态/配音筛选
- 分页和 Bilibili 来源回链
- 30 分钟新鲜缓存和 24 小时旧缓存降级
- 来源状态、连接测试和结构化错误提示
- 桌面端和移动端响应式布局

## 当前限制

- 各平台字段和分页能力由上游接口决定，部分平台的筛选项仍是基础映射
- 还不能调用 Mopie 媒体识别、资源搜索和订阅 Tool
- 不提供自动订阅、平台登录、Cookie、SESSDATA 或 WBI
- 海报目前通过页面 CSP 白名单直接加载，通用安全图片服务仍待宿主扩展
- 主导航暂时通过 Mopie 前端路由显式接入，通用 manifest 导航注册仍待宿主实现

## Runtime

插件使用 Mopie 当前的 JSON-RPC over stdio 协议：

- `initialize`
- `health.check`
- `tool.call`
- `shutdown`

普通日志不能写入 stdout。插件数据由宿主传入 `initialize.config`，缓存写入宿主分配的 cache 目录。

## 参考与致谢

接口和分类设计参考：

- `KoWming/MoviePilot-Plugins` 的 `plugins.v2/exploreservices`
- `DDS-Derek/MoviePilot-Plugins` 的 `plugins.v3/bilibilidiscover`

本插件不复制 MoviePilot 内部 API 或数据库结构。参考仓库使用 GPL-3.0；如未来直接复用其代码，必须重新确认许可证和源码提供义务。
