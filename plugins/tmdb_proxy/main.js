// TMDB Proxy plugin for Mopie (JavaScript/goja)

var VERCEL_DEPLOY_URL = "https://vercel.com/import/project?template=https://github.com/imaliang/tmdb-proxy";

function buildTmdbUrl(path) {
    var config = ctx.config;
    var apiKey = config.api_key || "";
    var mode = config.proxy_mode || "vercel";

    if (mode === "vercel") {
        var domain = config.vercel_domain || "";
        if (!domain) {
            return { error: "未配置 Vercel 域名" };
        }
        domain = domain.replace(/\/+$/, "");
        if (domain.indexOf("http") !== 0) {
            domain = "https://" + domain;
        }
        return { url: domain + "/3" + path + "?api_key=" + apiKey };
    } else {
        var proxy = config.http_proxy || "";
        return { url: "https://api.themoviedb.org/3" + path + "?api_key=" + apiKey, proxy: proxy };
    }
}

function test_connection() {
    var config = ctx.config;
    var apiKey = config.api_key || "";
    if (!apiKey) {
        return { success: false, message: "未配置 API Key，请先在配置中填写" };
    }

    var urlInfo = buildTmdbUrl("/movie/550");
    if (urlInfo.error) {
        return { success: false, message: urlInfo.error };
    }

    var start = Date.now();
    var resp = ctx.http.get(urlInfo.url, {
        timeout: 10000,
        proxy: urlInfo.proxy || "",
        headers: { "Accept": "application/json" }
    });

    var elapsed = Date.now() - start;

    if (resp.status !== 200) {
        return { success: false, message: "❌ HTTP " + resp.status + ": " + (resp.error || resp.body) };
    }

    try {
        var data = JSON.parse(resp.body);
        if (data.id) {
            var mode = config.proxy_mode || "vercel";
            var modeText = mode === "vercel" ? "Vercel 反代" : "HTTP 代理";
            return {
                success: true,
                message: "✅ 连接正常（" + elapsed + "ms）\n方式: " + modeText + "\n测试影片: " + (data.title || "")
            };
        }
        return { success: false, message: "TMDB 返回异常数据" };
    } catch (e) {
        return { success: false, message: "❌ 解析响应失败: " + e.message };
    }
}

function deploy_vercel() {
    return {
        success: true,
        message: "即将跳转到 Vercel 部署页面...\n\n部署步骤:\n1. 登录 Vercel（免费注册）\n2. 确认部署\n3. 绑定自定义域名\n4. 回来填写域名并测试",
        data: { url: VERCEL_DEPLOY_URL }
    };
}

function sync_to_tmdb() {
    var config = ctx.config;
    var apiKey = config.api_key || "";
    var mode = config.proxy_mode || "vercel";

    var proxyValue = "";
    if (mode === "vercel") {
        var domain = config.vercel_domain || "";
        if (domain) {
            if (domain.indexOf("http") !== 0) {
                domain = "https://" + domain;
            }
            proxyValue = domain;
        }
    } else {
        proxyValue = config.http_proxy || "";
    }

    if (!apiKey) {
        return { success: false, message: "未配置 API Key" };
    }

    ctx.db.setSetting("tmdb_api_key", apiKey);
    ctx.db.setSetting("tmdb_proxy", proxyValue);

    return {
        success: true,
        message: "✅ 已同步到系统设置\nAPI Key: " + apiKey.substring(0, 8) + "...\n代理: " + (proxyValue || "无")
    };
}
