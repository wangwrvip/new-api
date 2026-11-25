#!/usr/bin/env python3
"""GLM Coding Plan 多账号用量监控。

用法：
    python glm-usage.py              # 单次查询
    python glm-usage.py --watch      # 每 60 秒刷新
    python glm-usage.py --watch 30   # 每 30 秒刷新

config.json 与本脚本同目录：
    {
      "accounts": [
        {"name": "Pro", "key": "xxx", "base": "cn"},
        {"name": "Lit", "key": "yyy", "base": "cn"}
      ]
    }

base 可选 "cn"（默认，open.bigmodel.cn）或 "intl"（api.z.ai）。

字段映射（基于 probe.py 探测结果）：
- TOKENS_LIMIT + nextResetTime 距当前 < 24h → 5 小时滚动窗口
- TOKENS_LIMIT + nextResetTime 距当前 > 24h → 周限额（仅新版套餐返回）
- TIME_LIMIT 不显示（按调用次数计的，与 token 限额不同维度）
"""
from __future__ import annotations

import argparse
import datetime as dt
import json
import os
import sys
import time
import urllib.error
import urllib.request
from concurrent.futures import ThreadPoolExecutor, as_completed

CN_BASE = "https://open.bigmodel.cn"
INTL_BASE = "https://api.z.ai"
PATH = "/api/monitor/usage/quota/limit"
DAY_MS = 24 * 3600 * 1000

YELLOW, RED, RESET = "\033[33m", "\033[31m", "\033[0m"

# Windows 控制台 UTF-8 输出 + VT 模式（让 ANSI 颜色码被正确解析）
if sys.platform == "win32":
    try:
        sys.stdout.reconfigure(encoding="utf-8")
    except AttributeError:
        pass
    try:
        import ctypes
        _k32 = ctypes.windll.kernel32
        _h = _k32.GetStdHandle(-11)  # STD_OUTPUT_HANDLE
        _mode = ctypes.c_uint32()
        if _k32.GetConsoleMode(_h, ctypes.byref(_mode)):
            _k32.SetConsoleMode(_h, _mode.value | 0x0004)  # ENABLE_VT_PROCESSING
    except Exception:
        pass


def load_config() -> list:
    p = os.path.join(os.path.dirname(os.path.abspath(__file__)), "config.json")
    if not os.path.exists(p):
        sys.exit(f"找不到 config.json: {p}")
    with open(p, encoding="utf-8") as f:
        cfg = json.load(f)
    if not cfg.get("accounts"):
        sys.exit("config.json 中没有 accounts")
    return cfg["accounts"]


def query_usage(acc: dict) -> dict:
    name = acc.get("name", "?")
    key = acc.get("key", "")
    base = INTL_BASE if acc.get("base", "cn") == "intl" else CN_BASE
    if not key:
        return {"name": name, "error": "missing key"}

    req = urllib.request.Request(
        base + PATH,
        headers={
            "Authorization": key,
            "Accept-Language": "en-US,en",
            "Content-Type": "application/json",
        },
        method="GET",
    )
    try:
        with urllib.request.urlopen(req, timeout=15) as r:
            payload = json.loads(r.read().decode("utf-8"))
        if not payload.get("success"):
            return {"name": name, "error": payload.get("msg", "unknown")}
        return _parse(name, payload.get("data") or {})
    except urllib.error.HTTPError as e:
        body = e.read().decode("utf-8", errors="replace")[:120]
        return {"name": name, "error": f"HTTP {e.code}: {body}"}
    except Exception as e:
        return {"name": name, "error": f"{type(e).__name__}: {e}"}


def _parse(name: str, data: dict) -> dict:
    """分类 TOKENS_LIMIT：nextResetTime 距当前 > 24h 视为周限额。"""
    now_ms = int(time.time() * 1000)
    out = {
        "name": name,
        "level": (data.get("level") or "?").upper(),
        "five_h": None,
        "weekly": None,
    }
    for lim in data.get("limits") or []:
        if lim.get("type") != "TOKENS_LIMIT":
            continue
        nr = lim.get("nextResetTime")
        if nr is None:
            continue
        entry = {
            "used_pct": lim.get("percentage", 0),
            "remaining_pct": 100 - lim.get("percentage", 0),
            "next_reset": nr,
        }
        slot = "weekly" if nr - now_ms > DAY_MS else "five_h"
        if out[slot] is None:
            out[slot] = entry
    return out


def _fmt_time(ms) -> str:
    if not ms:
        return "--"
    return dt.datetime.fromtimestamp(ms / 1000).strftime("%m-%d %H:%M")


def _pct_cell(remaining, used_pct: int, width: int) -> str:
    """百分比单元格：先按可见宽度对齐，再用颜色码包围（颜色码不影响布局）。"""
    if remaining is None:
        return "--".ljust(width)
    text = f"{remaining}%".ljust(width)
    if used_pct >= 95:
        return f"{RED}{text}{RESET}"
    if used_pct >= 80:
        return f"{YELLOW}{text}{RESET}"
    return text


def render(rows: list) -> None:
    headers = ["Name", "Plan", "5h Left", "5h Reset", "Wk Left", "Wk Reset"]
    widths = [12, 6, 9, 13, 9, 13]
    sep = "+" + "+".join("-" * (w + 2) for w in widths) + "+"

    def row(cells):
        return "| " + " | ".join(cells) + " |"

    print(sep)
    print(row([h.ljust(w) for h, w in zip(headers, widths)]))
    print(sep)
    for r in rows:
        if "error" in r:
            err = f"{RED}ERROR: {r['error']}{RESET}"
            print(f"| {r['name'].ljust(widths[0])} | {err}")
            continue
        five = r["five_h"] or {}
        wk = r["weekly"] or {}
        cells = [
            r["name"].ljust(widths[0]),
            r["level"].ljust(widths[1]),
            _pct_cell(five.get("remaining_pct"), five.get("used_pct", 0), widths[2]),
            _fmt_time(five.get("next_reset")).ljust(widths[3]),
            _pct_cell(wk.get("remaining_pct"), wk.get("used_pct", 0), widths[4]),
            _fmt_time(wk.get("next_reset")).ljust(widths[5]),
        ]
        print(row(cells))
    print(sep)


def _run_once(accounts: list) -> None:
    results: dict = {}
    with ThreadPoolExecutor(max_workers=min(10, len(accounts))) as ex:
        futs = {ex.submit(query_usage, a): i for i, a in enumerate(accounts)}
        for fut in as_completed(futs):
            i = futs[fut]
            try:
                results[i] = fut.result()
            except Exception as e:
                results[i] = {"name": accounts[i].get("name", "?"), "error": str(e)}
    print(f"更新时间: {dt.datetime.now().strftime('%Y-%m-%d %H:%M:%S')}")
    render([results[i] for i in range(len(accounts))])


def main() -> None:
    p = argparse.ArgumentParser(description="GLM Coding Plan 用量监控")
    p.add_argument(
        "--watch", nargs="?", const=60, type=int, default=None,
        help="每 N 秒刷新（默认 60）",
    )
    args = p.parse_args()

    accounts = load_config()

    if args.watch is None:
        _run_once(accounts)
        return

    try:
        while True:
            _run_once(accounts)
            print(f"\n下次刷新: {args.watch}s 后 (Ctrl+C 退出)")
            time.sleep(args.watch)
            os.system("cls" if os.name == "nt" else "clear")
    except KeyboardInterrupt:
        print("\n已退出")


if __name__ == "__main__":
    main()
