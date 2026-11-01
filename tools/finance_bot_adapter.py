#!/usr/bin/env python3
"""
finance_bot_adapter.py
Adapter CLI and Integration Tool for OpenClaw Bot and Finance Backend.

Supports:
- add_expense(amount, category, description, date, account)
- add_income(amount, category, description, date, account)
- get_summary(period, category, start_date, end_date)
- update_expense(transaction_id, amount, category, description, account)
- list_categories()
- list_accounts()

Requires Python 3.8+ (standard library only, no pip dependencies).
"""

import argparse
import datetime
import json
import os
import sys
import urllib.error
import urllib.parse
import urllib.request

DEFAULT_API_URL = os.environ.get("FINANCE_API_URL", "http://localhost:8080").rstrip("/")
API_KEY = os.environ.get("FINANCE_API_KEY") or os.environ.get("INGEST_API_KEY", "")
EMAIL = os.environ.get("FINANCE_EMAIL", "")
PASSWORD = os.environ.get("FINANCE_PASSWORD", "")
SESSION_CACHE_FILE = os.environ.get("FINANCE_SESSION_CACHE", "/tmp/finance_session_token.json")


class FinanceAPIClient:
    def __init__(self, base_url=DEFAULT_API_URL, api_key=API_KEY, email=EMAIL, password=PASSWORD):
        self.base_url = base_url
        self.api_key = api_key
        self.email = email
        self.password = password
        self.session_token = None
        self._categories_cache = None
        self._accounts_cache = None

    def _get_headers(self):
        headers = {"Content-Type": "application/json"}
        if self.api_key:
            headers["Authorization"] = f"Bearer {self.api_key}"
        elif self.session_token:
            headers["Cookie"] = f"finance_session={self.session_token}"
        return headers

    def _load_cached_session(self):
        if not os.path.exists(SESSION_CACHE_FILE):
            return None
        try:
            with open(SESSION_CACHE_FILE, "r") as f:
                data = json.load(f)
                expires_at = data.get("expires_at", 0)
                if expires_at > datetime.datetime.now(datetime.timezone.utc).timestamp():
                    return data.get("token")
        except Exception:
            pass
        return None

    def _save_cached_session(self, token, expires_at_timestamp):
        try:
            with open(SESSION_CACHE_FILE, "w") as f:
                json.dump({"token": token, "expires_at": expires_at_timestamp}, f)
        except Exception:
            pass

    def ensure_authenticated(self):
        if self.api_key:
            return  # Bearer token auth is active

        # Check existing cached session
        cached = self._load_cached_session()
        if cached:
            self.session_token = cached
            return

        if not self.email or not self.password:
            # Try to see if API allows without auth (e.g. Ingest key was supplied)
            return

        # Perform login
        login_url = f"{self.base_url}/api/v1/auth/login"
        payload = json.dumps({"email": self.email, "password": self.password}).encode("utf-8")
        req = urllib.request.Request(login_url, data=payload, headers={"Content-Type": "application/json"}, method="POST")
        try:
            with urllib.request.urlopen(req, timeout=10) as resp:
                cookie_header = resp.headers.get("Set-Cookie")
                if cookie_header:
                    import http.cookies
                    cookie = http.cookies.SimpleCookie()
                    cookie.load(cookie_header)
                    if "finance_session" in cookie:
                        self.session_token = cookie["finance_session"].value
                        # Cache for 29 days
                        exp = (datetime.datetime.now(datetime.timezone.utc) + datetime.timedelta(days=29)).timestamp()
                        self._save_cached_session(self.session_token, exp)
        except Exception as e:
            raise RuntimeError(f"Gagal login ke Finance API: {e}")

    def request(self, method, path, data=None):
        self.ensure_authenticated()
        url = f"{self.base_url}{path}"
        body = json.dumps(data).encode("utf-8") if data is not None else None
        headers = self._get_headers()
        req = urllib.request.Request(url, data=body, headers=headers, method=method)

        try:
            with urllib.request.urlopen(req, timeout=15) as resp:
                raw = resp.read().decode("utf-8")
                return json.loads(raw) if raw else {"success": True}
        except urllib.error.HTTPError as e:
            err_body = e.read().decode("utf-8") if e.fp else ""
            try:
                err_json = json.loads(err_body)
                err_msg = err_json.get("error", err_body)
            except Exception:
                err_msg = err_body or str(e)
            raise RuntimeError(f"HTTP {e.code}: {err_msg}")
        except urllib.error.URLError as e:
            raise RuntimeError(f"Koneksi gagal ke {url}: {e.reason}")

    def get_accounts(self):
        if self._accounts_cache is None:
            res = self.request("GET", "/api/v1/accounts")
            self._accounts_cache = res.get("data", [])
        return self._accounts_cache

    def get_categories(self):
        if self._categories_cache is None:
            res = self.request("GET", "/api/v1/categories")
            self._categories_cache = res.get("data", [])
        return self._categories_cache

    def resolve_account_id(self, account_identifier=None):
        accounts = self.get_accounts()
        if not accounts:
            raise RuntimeError("Belum ada data rekening (accounts) di database.")

        if not account_identifier:
            # Default to first active account
            for acc in accounts:
                if acc.get("is_active", True):
                    return acc["id"], acc["name"]
            return accounts[0]["id"], accounts[0]["name"]

        # Check if numeric ID
        try:
            acc_id = int(account_identifier)
            for acc in accounts:
                if acc["id"] == acc_id:
                    return acc["id"], acc["name"]
        except ValueError:
            pass

        # Match by name
        target = str(account_identifier).lower().strip()
        for acc in accounts:
            if acc["name"].lower() == target:
                return acc["id"], acc["name"]
        for acc in accounts:
            if target in acc["name"].lower():
                return acc["id"], acc["name"]

        # If not matched, use first account
        return accounts[0]["id"], accounts[0]["name"]

    def resolve_category_id(self, category_identifier, cat_type="expense"):
        categories = self.get_categories()

        # Check if numeric ID
        try:
            cat_id = int(category_identifier)
            for cat in categories:
                if cat["id"] == cat_id:
                    return cat["id"], cat["name"]
        except ValueError:
            pass

        target = str(category_identifier).lower().strip()

        # Exact match with matching type
        for cat in categories:
            if cat.get("type") == cat_type and cat["name"].lower() == target:
                return cat["id"], cat["name"]

        # Partial/contains match
        for cat in categories:
            if cat.get("type") == cat_type and target in cat["name"].lower():
                return cat["id"], cat["name"]

        # Keyword mapping shortcuts
        shortcuts = {
            "makan": "Makanan & Minuman",
            "minum": "Makanan & Minuman",
            "food": "Makanan & Minuman",
            "transport": "Transportasi",
            "bensin": "Transportasi",
            "gojek": "Transportasi",
            "grab": "Transportasi",
            "belanja": "Belanja",
            "shopping": "Belanja",
            "listrik": "Tagihan",
            "wifi": "Tagihan",
            "pulsa": "Tagihan",
            "obat": "Kesehatan",
            "dokter": "Kesehatan",
            "nonton": "Hiburan",
            "game": "Hiburan",
            "gaji": "Pemasukan",
            "income": "Pemasukan",
        }
        for kw, mapped in shortcuts.items():
            if kw in target:
                for cat in categories:
                    if cat.get("type") == cat_type and cat["name"].lower() == mapped.lower():
                        return cat["id"], cat["name"]

        # Fallback category
        fallback_name = "Belum Dikategorikan" if cat_type == "expense" else "Pemasukan"
        for cat in categories:
            if cat.get("type") == cat_type and cat["name"].lower() == fallback_name.lower():
                return cat["id"], cat["name"]

        # First category of type
        for cat in categories:
            if cat.get("type") == cat_type:
                return cat["id"], cat["name"]

        raise RuntimeError(f"Tidak dapat menemukan kategori untuk '{category_identifier}'.")


def parse_date(date_str=None):
    now = datetime.datetime.now(datetime.timezone(datetime.timedelta(hours=7)))  # Asia/Jakarta UTC+7
    if not date_str or date_str.lower() in ("today", "hari_ini", "sekarang"):
        return now.isoformat()
    if date_str.lower() in ("yesterday", "kemarin"):
        return (now - datetime.timedelta(days=1)).isoformat()
    try:
        # If YYYY-MM-DD
        dt = datetime.datetime.strptime(date_str, "%Y-%m-%d")
        dt = dt.replace(hour=now.hour, minute=now.minute, second=now.second, tzinfo=datetime.timezone(datetime.timedelta(hours=7)))
        return dt.isoformat()
    except ValueError:
        pass
    try:
        # If RFC3339 already
        return datetime.datetime.fromisoformat(date_str).isoformat()
    except Exception:
        return now.isoformat()


def resolve_date_range(period=None, start_date=None, end_date=None):
    now = datetime.datetime.now(datetime.timezone(datetime.timedelta(hours=7)))
    today_str = now.strftime("%Y-%m-%d")

    if start_date and end_date:
        return start_date, end_date

    period = (period or "today").lower().strip()

    if period in ("today", "hari_ini"):
        return today_str, today_str
    elif period in ("yesterday", "kemarin"):
        yesterday_str = (now - datetime.timedelta(days=1)).strftime("%Y-%m-%d")
        return yesterday_str, yesterday_str
    elif period in ("this_week", "minggu_ini"):
        # Monday of current week
        start_week = (now - datetime.timedelta(days=now.weekday())).strftime("%Y-%m-%d")
        return start_week, today_str
    elif period in ("last_week", "minggu_lalu"):
        start_last = (now - datetime.timedelta(days=now.weekday() + 7)).strftime("%Y-%m-%d")
        end_last = (now - datetime.timedelta(days=now.weekday() + 1)).strftime("%Y-%m-%d")
        return start_last, end_last
    elif period in ("this_month", "bulan_ini"):
        start_month = now.replace(day=1).strftime("%Y-%m-%d")
        return start_month, today_str
    elif period in ("last_month", "bulan_lalu"):
        first_current = now.replace(day=1)
        last_prev = first_current - datetime.timedelta(days=1)
        start_prev = last_prev.replace(day=1).strftime("%Y-%m-%d")
        return start_prev, last_prev.strftime("%Y-%m-%d")
    else:
        # Default to today
        return today_str, today_str


# ==========================================
# TOOL ACTIONS
# ==========================================

def action_add_expense(client: FinanceAPIClient, args):
    account_id, account_name = client.resolve_account_id(args.account)
    category_id, category_name = client.resolve_category_id(args.category, cat_type="expense")
    occurred_at = parse_date(args.date)

    payload = {
        "type": "expense",
        "amount": int(args.amount),
        "source_account_id": account_id,
        "category_id": category_id,
        "description": args.description or "",
        "occurred_at": occurred_at,
    }

    res = client.request("POST", "/api/v1/transactions", payload)
    data = res.get("data", {})

    output = {
        "success": True,
        "action": "add_expense",
        "transaction_id": data.get("id"),
        "amount": int(args.amount),
        "category": category_name,
        "account": account_name,
        "description": args.description,
        "occurred_at": data.get("occurred_at", occurred_at),
        "message": f"Pengeluaran Rp {int(args.amount):,} ({category_name}) berhasil dicatat ke rekening {account_name}."
    }
    print(json.dumps(output, ensure_ascii=False, indent=2))


def action_add_income(client: FinanceAPIClient, args):
    account_id, account_name = client.resolve_account_id(args.account)
    category_id, category_name = client.resolve_category_id(args.category or "Pemasukan", cat_type="income")
    occurred_at = parse_date(args.date)

    payload = {
        "type": "income",
        "amount": int(args.amount),
        "destination_account_id": account_id,
        "category_id": category_id,
        "description": args.description or "",
        "occurred_at": occurred_at,
    }

    res = client.request("POST", "/api/v1/transactions", payload)
    data = res.get("data", {})

    output = {
        "success": True,
        "action": "add_income",
        "transaction_id": data.get("id"),
        "amount": int(args.amount),
        "category": category_name,
        "account": account_name,
        "description": args.description,
        "occurred_at": data.get("occurred_at", occurred_at),
        "message": f"Pemasukan Rp {int(args.amount):,} ({category_name}) berhasil dicatat ke rekening {account_name}."
    }
    print(json.dumps(output, ensure_ascii=False, indent=2))


def action_get_summary(client: FinanceAPIClient, args):
    start_date, end_date = resolve_date_range(args.period, args.start_date, args.end_date)
    query_params = urllib.parse.urlencode({
        "start_date": start_date,
        "end_date": end_date,
        "comparison": "none"
    })
    res = client.request("GET", f"/api/v2/reports/statistics?{query_params}")
    report_data = res.get("data", {})
    summary = report_data.get("summary", {})
    categories = report_data.get("expense_by_category", [])

    # Filter category if requested
    if args.category:
        cat_filter = str(args.category).lower().strip()
        categories = [c for c in categories if cat_filter in c.get("category", "").lower()]

    output = {
        "success": True,
        "action": "get_summary",
        "period": f"{start_date} s/d {end_date}",
        "summary": {
            "total_expense": summary.get("expense", 0),
            "total_income": summary.get("income", 0),
            "net_cashflow": summary.get("net_cashflow", 0),
            "expense_transaction_count": summary.get("expense_transaction_count", 0),
            "average_daily_expense": summary.get("average_daily_expense", 0),
        },
        "expense_by_category": [
            {
                "category": c.get("category"),
                "amount": c.get("amount", 0),
                "percentage": round(c.get("percentage", 0.0), 2)
            }
            for c in categories
        ]
    }
    print(json.dumps(output, ensure_ascii=False, indent=2))


def action_update_expense(client: FinanceAPIClient, args):
    transaction_id = int(args.transaction_id)
    # Fetch existing transaction
    try:
        current_res = client.request("GET", f"/api/v1/transactions/{transaction_id}")
        current = current_res.get("data", {})
    except Exception as e:
        output = {"success": False, "error": f"Transaksi #{transaction_id} tidak ditemukan: {e}"}
        print(json.dumps(output, ensure_ascii=False, indent=2))
        sys.exit(1)

    new_amount = int(args.amount) if args.amount is not None else None
    existing_amount = current.get("amount")
    
    # Resolve category if provided
    new_category_id = None
    category_name = None
    if args.category:
        cat_type = current.get("type", "expense")
        new_category_id, category_name = client.resolve_category_id(args.category, cat_type=cat_type)

    # Resolve account if provided
    new_source_account_id = None
    if args.account:
        new_source_account_id, _ = client.resolve_account_id(args.account)

    new_desc = args.description if args.description is not None else current.get("description")

    # If amount is changed: perform delete and recreate to preserve ledger contract
    if new_amount is not None and new_amount != existing_amount:
        # Delete old
        client.request("DELETE", f"/api/v1/transactions/{transaction_id}")

        create_payload = {
            "type": current.get("type", "expense"),
            "amount": new_amount,
            "source_account_id": new_source_account_id or current.get("source_account_id"),
            "destination_account_id": current.get("destination_account_id"),
            "category_id": new_category_id or current.get("category_id"),
            "description": new_desc,
            "occurred_at": current.get("occurred_at")
        }
        res = client.request("POST", "/api/v1/transactions", create_payload)
        new_data = res.get("data", {})

        output = {
            "success": True,
            "action": "update_expense",
            "old_transaction_id": transaction_id,
            "new_transaction_id": new_data.get("id"),
            "amount": new_amount,
            "category": category_name or "Tercatat",
            "description": new_desc,
            "message": f"Koreksi transaksi #{transaction_id} berhasil: dibuat transaksi baru #{new_data.get('id')} dengan nominal Rp {new_amount:,}."
        }
        print(json.dumps(output, ensure_ascii=False, indent=2))
        return

    # If amount is not changed, use standard PATCH endpoint
    patch_payload = {}
    if new_category_id:
        patch_payload["category_id"] = new_category_id
    if new_desc is not None:
        patch_payload["description"] = new_desc
    if new_source_account_id:
        patch_payload["source_account_id"] = new_source_account_id

    if not patch_payload:
        output = {"success": True, "action": "update_expense", "message": "Tidak ada perubahan data."}
        print(json.dumps(output, ensure_ascii=False, indent=2))
        return

    res = client.request("PATCH", f"/api/v1/transactions/{transaction_id}", patch_payload)
    output = {
        "success": True,
        "action": "update_expense",
        "transaction_id": transaction_id,
        "updated_fields": list(patch_payload.keys()),
        "message": f"Transaksi #{transaction_id} berhasil diperbarui."
    }
    print(json.dumps(output, ensure_ascii=False, indent=2))


def action_delete_expense(client: FinanceAPIClient, args):
    transaction_id = int(args.transaction_id)
    client.request("DELETE", f"/api/v1/transactions/{transaction_id}")
    output = {
        "success": True,
        "action": "delete_expense",
        "transaction_id": transaction_id,
        "message": f"Transaksi #{transaction_id} berhasil dihapus."
    }
    print(json.dumps(output, ensure_ascii=False, indent=2))


def action_list_categories(client: FinanceAPIClient, _):
    categories = client.get_categories()
    output = {
        "success": True,
        "categories": [
            {"id": c["id"], "name": c["name"], "type": c.get("type")}
            for c in categories
        ]
    }
    print(json.dumps(output, ensure_ascii=False, indent=2))


def action_list_accounts(client: FinanceAPIClient, _):
    accounts = client.get_accounts()
    output = {
        "success": True,
        "accounts": [
            {"id": a["id"], "name": a["name"], "type": a.get("type"), "balance": a.get("balance", 0)}
            for a in accounts
        ]
    }
    print(json.dumps(output, ensure_ascii=False, indent=2))


# ==========================================
# CLI ENTRYPOINT
# ==========================================

def main():
    parser = argparse.ArgumentParser(description="Finance Bot Adapter for OpenClaw")
    subparsers = parser.add_subparsers(dest="command", required=True)

    # add-expense
    p_add = subparsers.add_parser("add_expense", help="Tambah pengeluaran baru")
    p_add.add_argument("--amount", "-a", type=int, required=True, help="Nominal pengeluaran")
    p_add.add_argument("--category", "-c", type=str, required=True, help="Kategori pengeluaran (misal: 'Makanan & Minuman')")
    p_add.add_argument("--description", "-d", type=str, default="", help="Catatan / keterangan pengeluaran")
    p_add.add_argument("--date", type=str, default="today", help="Tanggal pengeluaran (today, yesterday, YYYY-MM-DD)")
    p_add.add_argument("--account", type=str, default=None, help="Rekening sumber (misal: SeaBank, ShopeePay)")

    # add-income
    p_inc = subparsers.add_parser("add_income", help="Tambah pemasukan baru")
    p_inc.add_argument("--amount", "-a", type=int, required=True, help="Nominal pemasukan")
    p_inc.add_argument("--category", "-c", type=str, default="Pemasukan", help="Kategori pemasukan")
    p_inc.add_argument("--description", "-d", type=str, default="", help="Keterangan pemasukan")
    p_inc.add_argument("--date", type=str, default="today", help="Tanggal pemasukan")
    p_inc.add_argument("--account", type=str, default=None, help="Rekening tujuan")

    # get-summary
    p_sum = subparsers.add_parser("get_summary", help="Ambil ringkasan keuangan")
    p_sum.add_argument("--period", "-p", type=str, default="today", choices=["today", "yesterday", "this_week", "last_week", "this_month", "last_month", "custom"], help="Periode rekap")
    p_sum.add_argument("--category", "-c", type=str, default=None, help="Filter per kategori tertentu")
    p_sum.add_argument("--start-date", type=str, default=None, help="Tanggal awal (YYYY-MM-DD)")
    p_sum.add_argument("--end-date", type=str, default=None, help="Tanggal akhir (YYYY-MM-DD)")

    # update-expense
    p_upd = subparsers.add_parser("update_expense", help="Koreksi transaksi")
    p_upd.add_argument("--transaction-id", "--id", type=int, required=True, help="ID Transaksi")
    p_upd.add_argument("--amount", "-a", type=int, default=None, help="Nominal baru (opsional)")
    p_upd.add_argument("--category", "-c", type=str, default=None, help="Kategori baru (opsional)")
    p_upd.add_argument("--description", "-d", type=str, default=None, help="Keterangan baru (opsional)")
    p_upd.add_argument("--account", type=str, default=None, help="Rekening sumber baru (opsional)")

    # delete-expense
    p_del = subparsers.add_parser("delete_expense", help="Hapus transaksi")
    p_del.add_argument("--transaction-id", "--id", type=int, required=True, help="ID Transaksi")

    # list categories & accounts
    subparsers.add_parser("list_categories", help="Daftar kategori")
    subparsers.add_parser("list_accounts", help="Daftar rekening")

    args = parser.parse_args()
    client = FinanceAPIClient()

    try:
        commands = {
            "add_expense": action_add_expense,
            "add_income": action_add_income,
            "get_summary": action_get_summary,
            "update_expense": action_update_expense,
            "delete_expense": action_delete_expense,
            "list_categories": action_list_categories,
            "list_accounts": action_list_accounts,
        }
        cmd_func = commands.get(args.command)
        if cmd_func:
            cmd_func(client, args)
        else:
            parser.print_help()
    except Exception as e:
        error_output = {
            "success": False,
            "error": str(e)
        }
        print(json.dumps(error_output, ensure_ascii=False, indent=2))
        sys.exit(1)


if __name__ == "__main__":
    main()
