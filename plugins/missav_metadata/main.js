// MissAV Metadata plugin for Mopie (JavaScript/goja)
// Simple: open page + view trending via FlareSolverr

var DEFAULT_UA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36";

// Fetch via FlareSolverr (不传proxy - 走代理会被Cloudflare识别为自动化请求)
function fetchViaFlare(flaresolverrUrl, targetUrl, proxy, timeout) {
    var reqBody = { cmd: "request.get", url: targetUrl, maxTimeout: timeout || 50000 };
    // 不传proxy给FlareSolverr，直接连接才能过Cloudflare

    var resp = ctx.http.post(flaresolverrUrl, {
        timeout: (timeout || 50000) + 10000,
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(reqBody)
    });

    if (resp.status !== 200) return null;
    try {
        var data = JSON.parse(resp.body);
        if (data.status === "ok" && data.solution && data.solution.status === 200) {
            return data.solution.response;
        }
    } catch (e) {}
    return null;
}

// Parse video cards from HTML
function parseCards(body) {
    var results = [];
    // Match: <a href="/en/xxx">...<img data-src="cover" alt="title">...</a>
    var pattern = /<a[^>]*href="((?:\/[a-z]{2}\/)[^"]*?)"[^>]*>\s*<img[^>]*data-src="([^"]*)"[^>]*alt="([^"]*)"[^>]*>/g;
    var m;
    while ((m = pattern.exec(body)) !== null) {
        var url = m[1], cover = m[2], title = m[3].trim();
        if (title.length > 1 && url.indexOf("/search/") === -1 && url !== "#" && url !== "/") {
            results.push({ url: url, title: title.substring(0, 150), cover: cover });
        }
        if (results.length >= 30) break;
    }
    // Fallback
    if (results.length === 0) {
        var imgP = /<img[^>]*data-src="([^"]*)"[^>]*alt="([^"]*)"[^>]*/g;
        while ((m = imgP.exec(body)) !== null) {
            var alt = m[2].trim();
            if (alt.length > 2 && alt !== "English" && alt.indexOf("localizedUrl") === -1) {
                results.push({ url: "", title: alt.substring(0, 150), cover: m[1] });
            }
            if (results.length >= 30) break;
        }
    }
    return results;
}

// ============ Commands ============

function health() {
    var config = ctx.config;
    var flareUrl = config.flaresolverr_url || "";
    if (!flareUrl) {
        return { success: false, message: "请先配置 FlareSolverr 地址" };
    }
    var baseUrl = config.base_url || "https://missav.ws";
    var body = fetchViaFlare(flareUrl, baseUrl + "/en", null, 55000);
    if (body && body.length > 1000 && body.indexOf("Just a moment") === -1) {
        return { success: true, message: "✅ 连接正常（" + body.length + " 字节）" };
    }
    return { success: false, message: "❌ 无法连接" };
}

function trending() {
    var config = ctx.config;
    var flareUrl = config.flaresolverr_url || "";
    if (!flareUrl) {
        return { success: false, message: "请先配置 FlareSolverr 地址" };
    }
    var baseUrl = config.base_url || "https://missav.ws";
    var lang = config.language_prefix || "en";

    // Fetch new/trending page
    var url = baseUrl + "/" + lang + "/new";
    ctx.log("获取热榜:", url);

    var body = fetchViaFlare(flareUrl, url, null, 55000);
    if (!body) {
        return { success: false, message: "❌ 获取热榜失败" };
    }
    if (body.indexOf("Just a moment") !== -1) {
        return { success: false, message: "❌ 被 Cloudflare 拦截" };
    }

    var cards = parseCards(body);
    if (cards.length === 0) {
        return { success: false, message: "未解析到内容，页面结构可能已变化" };
    }

    // Format as numbered list
    var lines = [];
    for (var i = 0; i < cards.length; i++) {
        var c = cards[i];
        lines.push((i + 1 < 10 ? "0" : "") + (i + 1) + ". " + c.title);
    }

    return {
        success: true,
        message: "🔥 热榜（" + cards.length + " 条）\n\n" + lines.join("\n"),
        data: { cards: cards, count: cards.length }
    };
}
