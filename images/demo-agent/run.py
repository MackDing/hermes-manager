#!/usr/bin/env python3
"""Demo agent for HermesManager.

Reads task.json from /etc/hermesmanager/, sends lifecycle events to the
HermesManager callback URL, simulates work, and exits cleanly.
"""

import json
import os
import sys
import time
import urllib.request
import uuid


def post_event(callback_url: str, token: str, task_id: str, event_type: str, payload: dict) -> None:
    """POST a JSON event to the HermesManager callback URL."""
    event = {
        "id": f"evt-{uuid.uuid4()}",
        "task_id": task_id,
        "type": event_type,
        "payload": payload,
    }
    data = json.dumps(event).encode("utf-8")
    req = urllib.request.Request(
        callback_url,
        data=data,
        headers={
            "Content-Type": "application/json",
            "Authorization": f"Bearer {token}",
        },
        method="POST",
    )
    try:
        with urllib.request.urlopen(req, timeout=10) as resp:
            print(f"[demo-agent] {event_type} -> {resp.status}")
    except Exception as exc:
        print(f"[demo-agent] {event_type} failed: {exc}", file=sys.stderr)


def main() -> None:
    task_path = "/etc/hermesmanager/task.json"
    task_id = os.environ.get("HERMESMANAGER_TASK_ID", "")
    callback_url = os.environ.get("HERMESMANAGER_CALLBACK_URL", "")
    agent_token = os.environ.get("HERMESMANAGER_AGENT_TOKEN", "")

    if not task_id or not callback_url or not agent_token:
        print("[demo-agent] ERROR: missing required env vars", file=sys.stderr)
        sys.exit(1)

    # Read task definition.
    try:
        with open(task_path, "r") as f:
            task = json.load(f)
        print(f"[demo-agent] loaded task: {task.get('skill', 'unknown')}")
    except FileNotFoundError:
        print(f"[demo-agent] WARNING: {task_path} not found, continuing anyway", file=sys.stderr)
        task = {}

    # 1. Report task.started
    post_event(callback_url, agent_token, task_id, "task.started", {})

    # 2. Simulate work
    print("[demo-agent] working...")
    time.sleep(2)

    # 3. Report task.llm_call with mock data
    post_event(callback_url, agent_token, task_id, "task.llm_call", {
        "model": "demo-model",
        "prompt_tokens": 10,
        "completion_tokens": 5,
        "cost_usd": 0.001,
    })

    # 4. Report task.completed
    post_event(callback_url, agent_token, task_id, "task.completed", {
        "result": "Hello from demo agent!",
        "exit_code": 0,
    })

    print("[demo-agent] done")


if __name__ == "__main__":
    main()
