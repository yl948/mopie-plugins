# MissAV 元数据插件（设计样例）

> 状态：协议已实测，解析器待实现。2026-08-03 使用本地配置的 HTTP 代理访问成功：`/en/search/{query}?page=N` 和 `/en/{slug}` 返回 HTTP 200；默认首页与繁中无前缀路径可能触发 Cloudflare Managed Challenge。代理地址仅保存在本地配置，不提交仓库。

## 已验证的网站协议

- 基址：`https://missav.ws`，HTTP 会 301 到 HTTPS；
- 搜索：`GET /en/search/{URL编码关键词}?page=N`；
- 详情：搜索卡片提供 `GET /en/{slug}`；
- 分页：服务端输出当前页、总页数和 Next 链接；
- 搜索卡片：详情链接在 `a[href]`，标题在 `img[alt]`，封面在懒加载属性 `img[data-src]`；
- 详情元数据：`og:title`、`og:image`、`og:url`，以及演员、类型、制作商、导演和标签链接；
- `robots.txt` 允许公开搜索/详情抓取，但禁止 `/logout`、`/saved`；插件不会访问这些路径；
- 插件只请求 HTML 元数据和站点公开的短预览 MP4，不请求完整播放器、HLS 或下载链接。

## 定位

这是独立、显式启用的 NSFW 元数据提供器，不是 PT 站点插件：

- 支持番号、标题、演员关键词搜索；
- 读取封面、标题、番号、发布日期、时长、演员、标签和详情 URL；
- 数据只进入插件私有缓存；
- 默认不进入 Mopie 普通影视/PT 全局搜索；
- 仅按番号加载公开短预览 MP4；不抓取 HLS 地址、不下载完整视频，不绕过登录、地区或访问控制。

## 为什么需要独立代理

当前直连会被远端重置连接。插件配置必须包含：

- `base_url`：域名可能变化，不能写死；
- `proxy`：独立 HTTP/SOCKS 代理，不复用或修改 TMDB 配置；
- `user_agent`；
- `request_delay_ms`；
- `cache_minutes`。

代理 URL 应作为敏感配置掩码保存。客户端必须拒绝把 `base_url` 解析到私网、回环或链路本地地址，防止 SSRF。

## 命令

### health

只验证 DNS/TLS/HTTP 状态和最终域名，不下载媒体内容。

### search

输入：

```json
{"query":"番号或关键词","page":1}
```

输出：

```json
{
  "items": [
    {
      "provider": "missav",
      "externalId": "站点稳定ID或规范化番号",
      "code": "番号",
      "title": "标题",
      "coverUrl": "https://...",
      "detailUrl": "https://...",
      "actors": [],
      "tags": [],
      "durationMinutes": 0,
      "publishedAt": null
    }
  ],
  "page": 1,
  "hasNext": false
}
```

### detail

输入详情 URL 或番号，返回同一结构的完整字段。详情 URL 必须与已配置 `base_url` 同源或属于显式镜像白名单。

## 实施顺序

1. 通过插件级代理取得搜索页和详情页；
2. 保存脱敏 HTML 到 `testdata/search.html`、`testdata/detail.html`；
3. 先写 fixture 解析失败测试；
4. 实现纯 HTML parser，不让 parser 直接联网；
5. 实现带超时、限流、代理、重试和缓存的 client；
6. 用 `health/search/detail` runner 组合 client + parser；
7. 页面结构变化时只更新 parser fixture，不改核心 Mopie；
8. 最后再决定是否提供显式的“加入元数据资料库”，默认关闭。

## 运行时依赖

目标是 Python runner：

- `httpx`：代理、超时和重定向控制；
- `selectolax`：HTML 解析；
- `mopie_api.config/log/storage`：配置、日志和私有缓存。

在 Mopie Python runner 和 `network.http` Host API 发布前，本插件保持 Draft。

## 需要的下一份证据

任选其一即可继续实现解析器：

- 能访问该站的 HTTP/SOCKS 代理地址（在插件配置中使用，不提交仓库）；
- 搜索结果页和详情页的脱敏 HTML；
- 浏览器“另存为网页”文件，移除 Cookie、账号和个人信息。
