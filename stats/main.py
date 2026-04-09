#!/usr/bin/env python3
"""
GitHub issues statistics by week/month.

Shows average issue lifetime in days for:
- avg_open_days: average days spent in open state inside each period
- avg_closed_days: average days spent in closed state inside each period

Aggregation is done by actual time overlap with each period.
"""

from __future__ import annotations

import argparse
import datetime as dt
import json
import re
import sys
import urllib.error
import urllib.parse
import urllib.request
from collections import defaultdict
from dataclasses import dataclass
from typing import Dict, Iterable, List, Optional


GITHUB_API = "https://api.github.com"


@dataclass
class PeriodStats:
    open_days_sum: float = 0.0
    open_avg_count: int = 0
    closed_days_sum: float = 0.0
    closed_avg_count: int = 0
    in_pr_days_sum: float = 0.0
    in_pr_avg_count: int = 0
    open_count: int = 0
    closed_count: int = 0
    in_pr_count: int = 0

    def as_row(self, period: str) -> Dict[str, object]:
        avg_open = self.open_days_sum / self.open_avg_count if self.open_avg_count else None
        avg_closed = self.closed_days_sum / self.closed_avg_count if self.closed_avg_count else None
        avg_in_pr = self.in_pr_days_sum / self.in_pr_avg_count if self.in_pr_avg_count else None
        return {
            "period": period,
            "avg_open_days": round(avg_open, 3) if avg_open is not None else None,
            "avg_closed_days": round(avg_closed, 3) if avg_closed is not None else None,
            "avg_in_pr_days": round(avg_in_pr, 3) if avg_in_pr is not None else None,
            "open_issues_count": self.open_count,
            "closed_issues_count": self.closed_count,
            "in_pr_issues_count": self.in_pr_count,
        }


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description=(
            "Build statistics for GitHub issues: average lifetime in days by week/month."
        )
    )
    parser.add_argument("--owner", required=True, help="Repository owner")
    parser.add_argument("--repo", required=True, help="Repository name")
    parser.add_argument(
        "--period",
        choices=["week", "month"],
        default="week",
        help="Aggregation period",
    )
    parser.add_argument(
        "--state",
        choices=["all", "open", "closed"],
        default="all",
        help="Issue state filter for API request",
    )
    parser.add_argument(
        "--since",
        default=None,
        help="Only issues updated at or after this time (ISO 8601, e.g. 2026-01-01T00:00:00Z)",
    )
    parser.add_argument(
        "--token",
        default=None,
        help="GitHub token (optional). If not set, uses GITHUB_TOKEN env var when present.",
    )
    parser.add_argument(
        "--pr-wait-method",
        choices=["auto", "graphql", "rest", "none"],
        default="auto",
        help=(
            "How to detect issue entering PR waiting stage: "
            "graphql timeline, rest closing keywords, auto fallback, or none"
        ),
    )
    return parser.parse_args()


def parse_github_datetime(value: str) -> dt.datetime:
    return dt.datetime.strptime(value, "%Y-%m-%dT%H:%M:%SZ").replace(
        tzinfo=dt.timezone.utc
    )


def period_key(created_at: dt.datetime, period: str) -> str:
    if period == "week":
        year, week, _ = created_at.isocalendar()
        return f"{year}-W{week:02d}"
    if period == "month":
        return created_at.strftime("%Y-%m")
    raise ValueError(f"Unsupported period: {period}")


def period_floor(value: dt.datetime, period: str) -> dt.datetime:
    value = value.astimezone(dt.timezone.utc)
    if period == "week":
        start = value - dt.timedelta(days=value.weekday())
        return start.replace(hour=0, minute=0, second=0, microsecond=0)
    if period == "month":
        return value.replace(day=1, hour=0, minute=0, second=0, microsecond=0)
    raise ValueError(f"Unsupported period: {period}")


def next_period_start(value: dt.datetime, period: str) -> dt.datetime:
    if period == "week":
        return value + dt.timedelta(days=7)
    if period == "month":
        if value.month == 12:
            return value.replace(year=value.year + 1, month=1, day=1)
        return value.replace(month=value.month + 1, day=1)
    raise ValueError(f"Unsupported period: {period}")


def split_interval_by_period(
    start: dt.datetime, end: dt.datetime, period: str
) -> Iterable[tuple[str, float]]:
    if end <= start:
        return

    current = start
    while current < end:
        bucket_start = period_floor(current, period)
        bucket_end = next_period_start(bucket_start, period)
        part_end = min(end, bucket_end)
        part_days = (part_end - current).total_seconds() / 86400
        yield period_key(bucket_start, period), part_days
        current = part_end


def build_issues_url(owner: str, repo: str, state: str, per_page: int, page: int, since: Optional[str]) -> str:
    params = {
        "state": state,
        "per_page": str(per_page),
        "page": str(page),
        "sort": "created",
        "direction": "asc",
    }
    if since:
        params["since"] = since
    query = urllib.parse.urlencode(params)
    return f"{GITHUB_API}/repos/{owner}/{repo}/issues?{query}"


def build_pulls_url(owner: str, repo: str, state: str, per_page: int, page: int) -> str:
    params = {
        "state": state,
        "per_page": str(per_page),
        "page": str(page),
        "sort": "created",
        "direction": "asc",
    }
    query = urllib.parse.urlencode(params)
    return f"{GITHUB_API}/repos/{owner}/{repo}/pulls?{query}"


def github_request_json(
    url: str,
    token: Optional[str],
    method: str = "GET",
    body: Optional[Dict[str, object]] = None,
) -> object:
    data = None
    if body is not None:
        data = json.dumps(body).encode("utf-8")
    req = urllib.request.Request(url, data=data, method=method)
    req.add_header("Accept", "application/vnd.github+json")
    req.add_header("User-Agent", "issue-stats-script")
    if body is not None:
        req.add_header("Content-Type", "application/json")
    if token:
        req.add_header("Authorization", f"Bearer {token}")

    try:
        with urllib.request.urlopen(req) as response:
            return json.loads(response.read().decode("utf-8"))
    except urllib.error.HTTPError as exc:
        body_text = exc.read().decode("utf-8", errors="replace")
        raise RuntimeError(f"GitHub API error {exc.code} for {url}\n{body_text}") from exc
    except urllib.error.URLError as exc:
        raise RuntimeError(f"Network error while calling GitHub API: {exc}") from exc


def fetch_issues(owner: str, repo: str, state: str, token: Optional[str], since: Optional[str]) -> Iterable[Dict[str, object]]:
    page = 1
    per_page = 100

    while True:
        url = build_issues_url(owner, repo, state, per_page, page, since)
        payload = github_request_json(url, token=token)
        if not isinstance(payload, list):
            raise RuntimeError(f"Unexpected GitHub response shape for {url}")

        if not payload:
            break

        for item in payload:
            # Pull requests are also returned by this endpoint.
            if "pull_request" in item:
                continue
            yield item

        page += 1


def fetch_pull_requests(owner: str, repo: str, token: Optional[str]) -> Iterable[Dict[str, object]]:
    page = 1
    per_page = 100
    while True:
        url = build_pulls_url(owner, repo, state="all", per_page=per_page, page=page)
        payload = github_request_json(url, token=token)
        if not isinstance(payload, list):
            raise RuntimeError(f"Unexpected GitHub response shape for {url}")
        if not payload:
            break
        for item in payload:
            yield item
        page += 1


def parse_linked_issue_numbers(text: str, owner: str, repo: str) -> set[int]:
    # Handles patterns like:
    # - closes #123
    # - fixes owner/repo#123
    # - resolved https://github.com/owner/repo/issues/123
    pattern = re.compile(
        r"(?i)\b(close[sd]?|fix(e[sd])?|resolve[sd]?)\s+"
        r"(?:"
        r"#(?P<local>\d+)|"
        r"(?P<full_repo>[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+)#(?P<full_num>\d+)|"
        r"https?://github\.com/(?P<url_owner>[A-Za-z0-9_.-]+)/(?P<url_repo>[A-Za-z0-9_.-]+)/issues/(?P<url_num>\d+)"
        r")"
    )
    issue_numbers: set[int] = set()
    for match in pattern.finditer(text or ""):
        local = match.group("local")
        if local:
            issue_numbers.add(int(local))
            continue

        full_repo = match.group("full_repo")
        full_num = match.group("full_num")
        if full_repo and full_num and full_repo.lower() == f"{owner}/{repo}".lower():
            issue_numbers.add(int(full_num))
            continue

        url_owner = match.group("url_owner")
        url_repo = match.group("url_repo")
        url_num = match.group("url_num")
        if url_owner and url_repo and url_num:
            if f"{url_owner}/{url_repo}".lower() == f"{owner}/{repo}".lower():
                issue_numbers.add(int(url_num))
    return issue_numbers


def build_rest_in_pr_index(owner: str, repo: str, token: Optional[str]) -> Dict[int, dt.datetime]:
    in_pr_at: Dict[int, dt.datetime] = {}
    for pr in fetch_pull_requests(owner, repo, token):
        created_raw = pr.get("created_at")
        if not created_raw:
            continue
        created_at = parse_github_datetime(str(created_raw))
        body = str(pr.get("body") or "")
        title = str(pr.get("title") or "")
        linked_issues = parse_linked_issue_numbers(f"{title}\n{body}", owner, repo)
        for issue_number in linked_issues:
            previous = in_pr_at.get(issue_number)
            if previous is None or created_at < previous:
                in_pr_at[issue_number] = created_at
    return in_pr_at


def graphql_issue_in_pr_at(
    owner: str, repo: str, issue_number: int, token: Optional[str]
) -> Optional[dt.datetime]:
    query = """
    query($owner: String!, $repo: String!, $number: Int!, $cursor: String) {
      repository(owner: $owner, name: $repo) {
        issue(number: $number) {
          timelineItems(
            first: 100,
            after: $cursor,
            itemTypes: [CONNECTED_EVENT, CROSS_REFERENCED_EVENT]
          ) {
            nodes {
              __typename
              ... on ConnectedEvent {
                createdAt
                subject {
                  __typename
                  ... on PullRequest {
                    createdAt
                  }
                }
              }
              ... on CrossReferencedEvent {
                createdAt
                source {
                  __typename
                  ... on PullRequest {
                    createdAt
                  }
                }
              }
            }
            pageInfo {
              hasNextPage
              endCursor
            }
          }
        }
      }
    }
    """
    cursor: Optional[str] = None
    earliest: Optional[dt.datetime] = None
    while True:
        payload = github_request_json(
            f"{GITHUB_API}/graphql",
            token=token,
            method="POST",
            body={
                "query": query,
                "variables": {
                    "owner": owner,
                    "repo": repo,
                    "number": issue_number,
                    "cursor": cursor,
                },
            },
        )
        if not isinstance(payload, dict):
            raise RuntimeError("Unexpected GraphQL response shape.")
        if payload.get("errors"):
            raise RuntimeError(f"GraphQL errors: {payload.get('errors')}")

        data = payload.get("data", {})
        repository = data.get("repository") if isinstance(data, dict) else None
        issue = repository.get("issue") if isinstance(repository, dict) else None
        timeline = issue.get("timelineItems") if isinstance(issue, dict) else None
        if not isinstance(timeline, dict):
            return earliest
        nodes = timeline.get("nodes", [])
        if not isinstance(nodes, list):
            nodes = []

        for node in nodes:
            if not isinstance(node, dict):
                continue
            node_type = node.get("__typename")
            pr_created_at_raw: Optional[str] = None
            if node_type == "ConnectedEvent":
                subject = node.get("subject")
                if isinstance(subject, dict) and subject.get("__typename") == "PullRequest":
                    pr_created_at_raw = subject.get("createdAt")
            elif node_type == "CrossReferencedEvent":
                source = node.get("source")
                if isinstance(source, dict) and source.get("__typename") == "PullRequest":
                    pr_created_at_raw = source.get("createdAt")
            if not pr_created_at_raw:
                continue

            candidate = parse_github_datetime(str(pr_created_at_raw))
            if earliest is None or candidate < earliest:
                earliest = candidate

        page_info = timeline.get("pageInfo", {})
        if not isinstance(page_info, dict) or not page_info.get("hasNextPage"):
            break
        cursor = page_info.get("endCursor")
        if cursor is None:
            break
    return earliest


def calculate_stats(
    issues: Iterable[Dict[str, object]],
    period: str,
    owner: str,
    repo: str,
    token: Optional[str],
    pr_wait_method: str,
) -> List[Dict[str, object]]:
    now = dt.datetime.now(dt.timezone.utc)
    grouped: Dict[str, PeriodStats] = defaultdict(PeriodStats)
    rest_in_pr_index: Optional[Dict[int, dt.datetime]] = None
    graphql_available = pr_wait_method in {"auto", "graphql"}
    rest_available = pr_wait_method in {"auto", "rest"}

    issues_list = list(issues)
    if rest_available:
        rest_in_pr_index = build_rest_in_pr_index(owner=owner, repo=repo, token=token)

    for issue in issues_list:
        created_raw = issue.get("created_at")
        if not created_raw:
            continue
        created_at = parse_github_datetime(str(created_raw))
        closed_raw = issue.get("closed_at")
        closed_at = parse_github_datetime(str(closed_raw)) if closed_raw else None

        # Open-state duration split by calendar periods.
        open_end = closed_at if closed_at is not None else now
        created_key = period_key(period_floor(created_at, period), period)
        grouped[created_key].open_count += 1

        for key, open_days in split_interval_by_period(created_at, open_end, period):
            grouped[key].open_days_sum += open_days
            grouped[key].open_avg_count += 1

        # Closed-state duration split by calendar periods.
        if closed_at is not None:
            closed_key = period_key(period_floor(closed_at, period), period)
            grouped[closed_key].closed_count += 1
            for key, closed_days in split_interval_by_period(closed_at, now, period):
                grouped[key].closed_days_sum += closed_days
                grouped[key].closed_avg_count += 1

        issue_number = issue.get("number")
        in_pr_at: Optional[dt.datetime] = None
        if pr_wait_method != "none" and issue_number is not None:
            if graphql_available:
                try:
                    in_pr_at = graphql_issue_in_pr_at(
                        owner=owner,
                        repo=repo,
                        issue_number=int(issue_number),
                        token=token,
                    )
                except RuntimeError as exc:
                    if pr_wait_method == "graphql":
                        raise
                    print(
                        f"Warning: GraphQL PR-link lookup failed for issue #{issue_number}: {exc}",
                        file=sys.stderr,
                    )
                    graphql_available = False

            if in_pr_at is None and rest_available and rest_in_pr_index is not None:
                in_pr_at = rest_in_pr_index.get(int(issue_number))

        if in_pr_at is not None:
            in_pr_key = period_key(period_floor(in_pr_at, period), period)
            grouped[in_pr_key].in_pr_count += 1
            pr_wait_end = closed_at if closed_at is not None else now
            if in_pr_at < pr_wait_end:
                for key, in_pr_days in split_interval_by_period(in_pr_at, pr_wait_end, period):
                    grouped[key].in_pr_days_sum += in_pr_days
                    grouped[key].in_pr_avg_count += 1

    rows = [grouped[k].as_row(k) for k in sorted(grouped.keys())]
    total_open = 0
    total_closed = 0
    for row in rows:
        total_open += int(row["open_issues_count"])
        total_closed += int(row["closed_issues_count"])
        row["total_open"] = total_open
        row["total_closed"] = total_closed
    return rows


def print_table(rows: List[Dict[str, object]], period: str) -> None:
    if not rows:
        print("No issues found for selected filters.")
        return

    headers = [
        period,
        "avg_open_days",
        "avg_closed_days",
        "avg_in_pr_days",
        "open_count",
        "closed_count",
        "in_pr_count",
        "total_open",
        "total_closed",
    ]

    data = [
        [
            str(r["period"]),
            "-" if r["avg_open_days"] is None else str(r["avg_open_days"]),
            "-" if r["avg_closed_days"] is None else str(r["avg_closed_days"]),
            "-" if r["avg_in_pr_days"] is None else str(r["avg_in_pr_days"]),
            str(r["open_issues_count"]),
            str(r["closed_issues_count"]),
            str(r["in_pr_issues_count"]),
            str(r["total_open"]),
            str(r["total_closed"]),
        ]
        for r in rows
    ]

    widths = [len(h) for h in headers]
    for row in data:
        for i, cell in enumerate(row):
            widths[i] = max(widths[i], len(cell))

    line = " | ".join(h.ljust(widths[i]) for i, h in enumerate(headers))
    sep = "-+-".join("-" * widths[i] for i in range(len(headers)))
    print(line)
    print(sep)
    for row in data:
        print(" | ".join(row[i].ljust(widths[i]) for i in range(len(headers))))


def main() -> int:
    args = parse_args()
    token = args.token
    if token is None:
        # Lazy import to keep module imports simple.
        import os

        token = os.getenv("GITHUB_TOKEN")

    try:
        issues = fetch_issues(
            owner=args.owner,
            repo=args.repo,
            state=args.state,
            token=token,
            since=args.since,
        )
        rows = calculate_stats(
            issues=issues,
            period=args.period,
            owner=args.owner,
            repo=args.repo,
            token=token,
            pr_wait_method=args.pr_wait_method,
        )
    except RuntimeError as exc:
        print(str(exc), file=sys.stderr)
        return 1

    print_table(rows, args.period)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
