var BASE_URL = "https://www.javbus.com";
var DEFAULT_PROXY = "";
var USER_AGENT = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36";

function requestCookie() {
    var fromConfig = ctx.config && ctx.config.cookie;
    var fromArgs = ctx.args && ctx.args.cookie;
    var value = String(fromConfig || fromArgs || "").trim();
    if (value.length > 4096) throw new Error("JavBus Cookie 格式无效。");
    value = value.split(/[;\r\n]+/).map(function (part) { return part.trim(); }).filter(Boolean).join("; ");
    if (/[\x00-\x1f\x7f]/.test(value)) throw new Error("JavBus Cookie 格式无效。");
    if (value && !/(?:^|;\s*)existmag=/i.test(value)) value += "; existmag=mag";
    if (!value) value = "existmag=mag";
    return value;
}

function decodeHtml(value) {
    return String(value || "")
        .replace(/&amp;/gi, "&")
        .replace(/&quot;/gi, '"')
        .replace(/&#39;|&apos;/gi, "'")
        .replace(/&lt;/gi, "<")
        .replace(/&gt;/gi, ">")
        .replace(/&#(\d+);/g, function (_, code) { return String.fromCharCode(Number(code)); })
        .replace(/&#x([\da-f]+);/gi, function (_, code) { return String.fromCharCode(parseInt(code, 16)); });
}

function textOnly(value) {
    return decodeHtml(String(value || "")
        .replace(/<script\b[^>]*>[\s\S]*?<\/script>/gi, " ")
        .replace(/<style\b[^>]*>[\s\S]*?<\/style>/gi, " ")
        .replace(/<[^>]+>/g, " ")
        .replace(/\s+/g, " ")).trim();
}

function attr(tag, name) {
    var escaped = name.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
    var match = new RegExp("\\b" + escaped + "\\s*=\\s*([\\\"'])(.*?)\\1", "i").exec(tag);
    return match ? decodeHtml(match[2]) : "";
}

function safeDetailUrl(value) {
    var raw = String(value || "").trim();
    if (!raw || raw.indexOf("\\") >= 0 || raw.indexOf("//") === 0) return "";
    if (raw.charAt(0) === "/") raw = BASE_URL + raw;
    if (!/^https:\/\/www\.javbus\.com(?:\/[^\s?#]*)?(?:[?#][^\s]*)?$/i.test(raw)) return "";
    return raw;
}

function fetchPage(url) {
    var response = ctx.http.get(url, {
        timeout: 25000,
        proxy: String(ctx.config.proxy || DEFAULT_PROXY).trim(),
        headers: {
            "User-Agent": USER_AGENT,
            "Accept": "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
            "Accept-Language": "zh-CN,zh;q=0.9,en;q=0.7",
            "Cookie": requestCookie(),
            "Referer": BASE_URL + "/"
        }
    });
    if (!response || response.status < 200 || response.status >= 300) {
        throw new Error(response && response.status ? "JavBus 返回 HTTP " + response.status : (response && response.error) || "请求失败");
    }
    var body = String(response.body || "");
    if (/driver-verify|Just a moment|challenge-platform/i.test(body) || /\/doc\/driver-verify/i.test(String(response.headers && response.headers.Location || ""))) {
        throw new Error("JavBus 返回访问验证页，当前请求无法继续；请先确认代理出口可正常访问该站。");
    }
    return body;
}

function absolutePic(src) {
    src = String(src || "").trim();
    if (src.indexOf("//") === 0) src = "https:" + src;
    if (src.charAt(0) === "/") src = BASE_URL + src;
    if (!/^https:\/\/www\.javbus\.com\/(?:pics\/|imgs\/(?:cover|thumbs)\/)[a-z0-9_./-]+$/i.test(src)) return "";
    return src;
}

function parseCards(html) {
    var results = [];
    var pattern = /<a\b[^>]*class="movie-box"[^>]*>[\s\S]*?<\/a>/gi;
    var match;
    while ((match = pattern.exec(html)) !== null && results.length < 40) {
        var block = match[0];
        var openTag = /^<a\b[^>]*>/i.exec(block);
        if (!openTag) continue;
        var url = safeDetailUrl(attr(openTag[0], "href"));
        if (!url) continue;
        var code = url.replace(/[?#].*$/, "").split("/").pop().toUpperCase();
        if (!/^[A-Z0-9][A-Z0-9_-]{2,40}$/.test(code)) continue;
        var imageTag = /<img\b[^>]*>/i.exec(block);
        var title = imageTag ? (attr(imageTag[0], "title") || attr(imageTag[0], "alt")) : "";
        var thumb = imageTag ? absolutePic(attr(imageTag[0], "src")) : "";
        var image = thumb;
        var thumbId = /\/pics\/thumb\/([a-z0-9]+)\.jpg$/i.exec(thumb);
        if (thumbId) image = BASE_URL + "/pics/cover/" + thumbId[1] + "_b.jpg";
        var uncId = /\/imgs\/thumbs\/([a-z0-9]+)\.jpg$/i.exec(thumb);
        if (uncId) image = BASE_URL + "/imgs/cover/" + uncId[1] + "_b.jpg";
        var dates = [];
        var dateMatch;
        var datePattern = /<date>([\s\S]*?)<\/date>/gi;
        while ((dateMatch = datePattern.exec(block)) !== null && dates.length < 2) dates.push(textOnly(dateMatch[1]));
        var tags = [];
        var tagMatch;
        var tagPattern = /<button\b[^>]*>([\s\S]*?)<\/button>/gi;
        while ((tagMatch = tagPattern.exec(block)) !== null && tags.length < 4) {
            var tag = textOnly(tagMatch[1]);
            if (tag) tags.push(tag);
        }
        if (!title) title = code;
        results.push({ code: code, title: title, date: dates.length > 1 ? dates[1] : "", tags: tags, image: image, thumb: thumb, url: url });
    }
    return results;
}

function fetchMagnets(pageHtml, pageUrl) {
    var gid = /var gid = (\d+);/.exec(pageHtml || "");
    if (!gid) return "";
    var uc = /var uc = (\d+);/.exec(pageHtml || "");
    var img = /var img = '([^']*)';/.exec(pageHtml || "");
    var query = "gid=" + encodeURIComponent(gid[1])
        + "&lang=zh&img=" + encodeURIComponent(img ? img[1] : "")
        + "&uc=" + encodeURIComponent(uc ? uc[1] : "0")
        + "&floor=" + Math.floor(Math.random() * 1000 + 1);
    var response = ctx.http.get(BASE_URL + "/ajax/uncledatoolsbyajax.php?" + query, {
        timeout: 25000,
        proxy: String(ctx.config.proxy || DEFAULT_PROXY).trim(),
        headers: {
            "User-Agent": USER_AGENT,
            "Accept": "*/*",
            "Accept-Language": "zh-CN,zh;q=0.9,en;q=0.7",
            "Cookie": requestCookie(),
            "Referer": pageUrl || (BASE_URL + "/"),
            "X-Requested-With": "XMLHttpRequest"
        }
    });
    if (!response || response.status < 200 || response.status >= 300) return "";
    return String(response.body || "");
}

function parseMagnets(html) {
    var magnets = [];
    var seen = {};
    var rowPattern = /<tr\b[^>]*>[\s\S]*?<\/tr>/gi;
    var row;
    while ((row = rowPattern.exec(html || "")) !== null && magnets.length < 30) {
        var block = row[0];
        var href = /href="(magnet:\?[^"]+)"/i.exec(block);
        if (!href) continue;
        var magnet = decodeHtml(href[1]).trim();
        if (seen[magnet] || magnet.indexOf("magnet:?xt=urn:btih:") !== 0) continue;
        seen[magnet] = true;
        var cells = [];
        var cellPattern = /<td\b[^>]*>([\s\S]*?)<\/td>/gi;
        var cell;
        while ((cell = cellPattern.exec(block)) !== null && cells.length < 3) cells.push(textOnly(cell[1]));
        magnets.push({ title: cells[0] || "磁力链接", size: cells[1] || "", date: cells[2] || "", uri: magnet });
    }
    return magnets;
}

function parseMetadata(html) {
    var labels = ["識別碼", "识别码", "發行日期", "发行日期", "長度", "长度", "導演", "导演", "製作商", "制作商", "發行商", "发行商", "類別", "类别", "演員", "演员"];
    var metadata = [];
    var seen = {};
    var pattern = /<p\b[^>]*>([\s\S]*?)<\/p>/gi;
    var match;
    while ((match = pattern.exec(html)) !== null) {
        var value = textOnly(match[1]);
        var colon = value.search(/[:：]/);
        if (colon <= 0) continue;
        var label = value.slice(0, colon).trim();
        if (labels.indexOf(label) < 0 || seen[label]) continue;
        seen[label] = true;
        var normalized = label.replace("發行日期", "发行日期").replace("長度", "长度").replace("導演", "导演").replace("製作商", "制作商").replace("發行商", "发行商").replace("類別", "类别").replace("演員", "演员");
        metadata.push({ label: normalized, value: value.slice(colon + 1).trim() });
    }
    return metadata;
}

function detectTitle(html, fallback) {
    var match = /<title\b[^>]*>([\s\S]*?)<\/title>/i.exec(html);
    if (match) return textOnly(match[1]).replace(/\s*[-|｜]\s*JavBus.*$/i, "");
    match = /<h3\b[^>]*>([\s\S]*?)<\/h3>/i.exec(html);
    return match ? textOnly(match[1]) : fallback;
}


function categoryURL(category, page) {
    category = String(category || "").trim();
    if (category && !/^(?:uncensored|genre\/[a-z0-9]+)$/.test(category)) throw new Error("分类无效");
    if (!category) return page <= 1 ? BASE_URL + "/" : BASE_URL + "/page/" + page;
    if (category === "uncensored") return page <= 1 ? BASE_URL + "/uncensored" : BASE_URL + "/uncensored/page/" + page;
    return page <= 1 ? BASE_URL + "/" + category : BASE_URL + "/" + category + "/" + page;
}

function latest() {
    var page = parseInt((ctx.args && ctx.args.page) || "1", 10);
    if (!page || page < 1) page = 1;
    if (page > 30) page = 30;
    try {
        var url = categoryURL(ctx.args && ctx.args.category, page);
        var results = parseCards(fetchPage(url));
        return { success: true, message: results.length ? "推荐 " + results.length + " 部" : "没有解析到推荐。", data: { results: results, page: page } };
    } catch (e) {
        return { success: false, message: e.message || String(e) };
    }
}

function genres() {
    try {
        var html = fetchPage(BASE_URL + "/genre");
        var groups = [];
        var parts = html.split(/<h4>/i);
        for (var i = 1; i < parts.length; i++) {
            var end = parts[i].indexOf("</h4>");
            if (end < 0) continue;
            var name = textOnly(parts[i].slice(0, end));
            var items = [];
            var seen = {};
            var pattern = /href="https:\/\/www\.javbus\.com\/genre\/([a-z0-9]+)"[^>]*>([\s\S]*?)<\/a>/gi;
            var match;
            while ((match = pattern.exec(parts[i])) !== null) {
                var label = textOnly(match[2]);
                if (!label || seen[match[1]]) continue;
                seen[match[1]] = true;
                items.push({ id: match[1], label: label });
            }
            if (name && items.length) groups.push({ name: name, items: items });
        }
        return { success: true, message: "分类 " + groups.length + " 组", data: { groups: groups } };
    } catch (e) {
        return { success: false, message: e.message || String(e) };
    }
}

function test_connection() {
    try {
        var html = fetchPage(BASE_URL + "/");
        return { success: true, message: "连接正常，收到 " + html.length + " 字节。" };
    } catch (e) {
        return { success: false, message: e.message || String(e) };
    }
}

function collectSearch(url) {
    try {
        return { results: parseCards(fetchPage(url)) };
    } catch (e) {
        return { error: e.message || String(e), results: [] };
    }
}

function search() {
    var query = String((ctx.args && (ctx.args.query || ctx.args.keyword)) || "").trim();
    if (!query) return { success: false, message: "请输入番号或关键词。" };
    if (query.length > 80) return { success: false, message: "搜索内容不能超过 80 个字符。" };
    var encoded = encodeURIComponent(query);
    var censored = collectSearch(BASE_URL + "/search/" + encoded);
    var uncensored = collectSearch(BASE_URL + "/uncensored/search/" + encoded + "&type=&parent=uc");
    var seen = {};
    var results = [];
    [censored.results, uncensored.results].forEach(function (list) {
        list.forEach(function (item) {
            if (seen[item.url]) return;
            seen[item.url] = true;
            results.push(item);
        });
    });
    if (!results.length && (censored.error || uncensored.error)) {
        return { success: false, message: censored.error || uncensored.error };
    }
    return { success: true, message: results.length ? "找到 " + results.length + " 部" : "没有结果。", data: { results: results } };
}

function detail() {
    var url = safeDetailUrl(String((ctx.args && (ctx.args.url || ctx.args.detailUrl)) || ""));
    if (!url) return { success: false, message: "影片详情地址无效。" };
    try {
        var html = fetchPage(url);
        var magnets = parseMagnets(fetchMagnets(html, url) || html);
        var code = url.replace(/[?#].*$/, "").split("/").pop().toUpperCase();
        var coverMatch = /var\s+img\s*=\s*'(\/pics\/cover\/[^']+)'/i.exec(html);
        var cover = coverMatch ? absolutePic(coverMatch[1]) : "";
        if (!cover) {
            coverMatch = /(\/imgs\/cover\/[a-z0-9]+_b\.jpg)/i.exec(html);
            if (coverMatch) cover = absolutePic(coverMatch[1]);
        }
        return {
            success: true,
            message: magnets.length ? "找到 " + magnets.length + " 条磁力链接。" : "已读取详情页，但没有解析到磁力链接。",
            data: { code: code, title: detectTitle(html, code), url: url, image: cover, metadata: parseMetadata(html), magnets: magnets }
        };
    } catch (e) {
        return { success: false, message: e.message || String(e) };
    }
}
