#!/usr/bin/env python3
"""TMDB Proxy plugin for Mopie"""
import json
import sys
import argparse
import urllib.request
import urllib.error
import sqlite3
import os
import time

DB_PATH = os.environ.get("MOPIE_DB", "/app/data/mopie.db")
VERCEL_DEPLOY_URL = "https://vercel.com/import/project?template=https://github.com/imaliang/tmdb-proxy"


def build_tmdb_url(path, config):
    """Build TMDB API URL based on proxy mode"""
    api_key = config.get("api_key", "")
    mode = config.get("proxy_mode", "vercel")

    if mode == "vercel":
        domain = config.get("vercel_domain", "")
        if not domain:
            return None, None, "未配置 Vercel 域名"
        domain = domain.rstrip("/")
        if not domain.startswith("http"):
            domain = "https://" + domain
        return f"{domain}/3{path}?api_key={api_key}", None, None
    else:
        proxy = config.get("http_proxy", "")
        return f"https://api.themoviedb.org/3{path}?api_key={api_key}", proxy or None, None


def test_connection(config):
    """Test TMDB API connection"""
    api_key = config.get("api_key", "")
    if not api_key:
        return {"success": False, "message": "未配置 API Key，请先在配置中填写"}

    url, proxy_url, err = build_tmdb_url("/movie/550", config)
    if err:
        return {"success": False, "message": err}

    start = time.time()
    try:
        req = urllib.request.Request(url)
        req.add_header("Accept", "application/json")

        proxy_handler = None
        if proxy_url:
            proxy_handler = urllib.request.ProxyHandler({"http": proxy_url, "https": proxy_url})

        opener = urllib.request.build_opener(proxy_handler) if proxy_handler else urllib.request.build_opener()
        resp = opener.open(req, timeout=10)
        data = json.loads(resp.read())
        elapsed = int((time.time() - start) * 1000)

        if data.get("id"):
            title = data.get("title", "")
            mode = config.get("proxy_mode", "vercel")
            mode_text = "Vercel 反代" if mode == "vercel" else "HTTP 代理"
            return {
                "success": True,
                "message": f"✅ 连接正常（{elapsed}ms）\n方式: {mode_text}\n测试影片: {title}",
            }
        return {"success": False, "message": "TMDB 返回异常数据"}
    except urllib.error.HTTPError as e:
        return {"success": False, "message": f"❌ HTTP {e.code}: {e.reason}"}
    except Exception as e:
        return {"success": False, "message": f"❌ 连接失败: {str(e)}"}


def deploy_vercel(config):
    """Return Vercel deploy URL"""
    return {
        "success": True,
        "message": f"即将跳转到 Vercel 部署页面...\n\n部署步骤:\n1. 登录 Vercel（免费注册）\n2. 确认部署\n3. 绑定自定义域名\n4. 回来填写域名并测试",
        "data": {"url": VERCEL_DEPLOY_URL},
    }


def sync_to_tmdb(config):
    """Sync proxy config to Mopie system settings"""
    api_key = config.get("api_key", "")
    mode = config.get("proxy_mode", "vercel")

    proxy_value = ""
    if mode == "vercel":
        domain = config.get("vercel_domain", "")
        if domain:
            if not domain.startswith("http"):
                domain = "https://" + domain
            proxy_value = domain
    else:
        proxy_value = config.get("http_proxy", "")

    if not api_key:
        return {"success": False, "message": "未配置 API Key"}

    try:
        conn = sqlite3.connect(DB_PATH)
        conn.execute(
            """INSERT INTO settings (key, value, updated_at) VALUES (?, ?, CURRENT_TIMESTAMP)
               ON CONFLICT(key) DO UPDATE SET value=excluded.value, updated_at=excluded.updated_at""",
            ("tmdb_api_key", api_key),
        )
        conn.execute(
            """INSERT INTO settings (key, value, updated_at) VALUES (?, ?, CURRENT_TIMESTAMP)
               ON CONFLICT(key) DO UPDATE SET value=excluded.value, updated_at=excluded.updated_at""",
            ("tmdb_proxy", proxy_value),
        )
        conn.commit()
        conn.close()
        return {
            "success": True,
            "message": f"✅ 已同步到系统设置\nAPI Key: {api_key[:8]}...\n代理: {proxy_value or '无'}",
        }
    except Exception as e:
        return {"success": False, "message": f"❌ 同步失败: {str(e)}"}


COMMANDS = {
    "test_connection": test_connection,
    "deploy_vercel": deploy_vercel,
    "sync_to_tmdb": sync_to_tmdb,
}


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--command", default="")
    # Collect all other --key value pairs as config
    args, remaining = parser.parse_known_args()

    # Parse remaining args as key-value pairs
    config = {}
    i = 0
    while i < len(remaining):
        if remaining[i].startswith("--"):
            key = remaining[i][2:]
            if i + 1 < len(remaining):
                config[key] = remaining[i + 1]
                i += 2
            else:
                config[key] = ""
                i += 1
        else:
            i += 1

    cmd = args.command
    if cmd in COMMANDS:
        result = COMMANDS[cmd](config)
    else:
        result = {"success": False, "message": f"未知命令: {cmd}"}

    print(json.dumps(result, ensure_ascii=False))


if __name__ == "__main__":
    main()
