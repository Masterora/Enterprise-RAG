from __future__ import annotations

import json
import os
import time
import uuid
from typing import Any
from urllib.error import HTTPError
from urllib.request import Request, urlopen

BASE_URL = os.getenv("E2E_API_URL", "http://localhost:9999")


def request(
    path: str,
    payload: dict[str, Any] | None = None,
    token: str = "",
    body: bytes | None = None,
    content_type: str = "application/json",
) -> dict[str, Any]:
    data = body if body is not None else json.dumps(payload or {}).encode()
    headers = {"Content-Type": content_type}
    if token:
        headers["Authorization"] = f"Bearer {token}"
    try:
        with urlopen(Request(BASE_URL + path, data=data, headers=headers), timeout=30) as response:
            return json.load(response)
    except HTTPError as exc:
        detail = exc.read().decode(errors="replace")
        raise RuntimeError(f"{path} returned HTTP {exc.code}: {detail}") from exc


def upload(token: str, subject_id: str) -> dict[str, Any]:
    boundary = "----enterprise-rag-e2e"
    content = "系统代号是 Project Atlas，用于验证完整的知识库问答链路。".encode()
    parts = [
        f"--{boundary}\r\nContent-Disposition: form-data; name=\"subject_id\"\r\n\r\n{subject_id}\r\n".encode(),
        (
            f"--{boundary}\r\nContent-Disposition: form-data; name=\"file\"; "
            'filename="e2e-knowledge.txt"\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n'
        ).encode()
        + content
        + b"\r\n",
        f"--{boundary}--\r\n".encode(),
    ]
    return request(
        "/api/documents/upload",
        token=token,
        body=b"".join(parts),
        content_type=f"multipart/form-data; boundary={boundary}",
    )


def start_stream_and_cancel(token: str, subject_id: str) -> str:
    payload = json.dumps(
        {
            "subject_id": subject_id,
            "query": "验证运行取消",
            "llm_provider": "openrouter",
            "llm_model": "openai/gpt-5.6-sol",
        }
    ).encode()
    stream_request = Request(
        BASE_URL + "/api/chat/stream",
        data=payload,
        headers={
            "Content-Type": "application/json",
            "Accept": "text/event-stream",
            "Authorization": f"Bearer {token}",
        },
    )
    with urlopen(stream_request, timeout=30) as response:
        event = ""
        data = ""
        for raw_line in response:
            line = raw_line.decode().strip()
            if line.startswith("event:"):
                event = line.removeprefix("event:").strip()
            elif line.startswith("data:"):
                data += line.removeprefix("data:").strip()
            elif not line and event:
                if event == "run":
                    run_id = json.loads(data)["run_id"]
                    cancelled = request(
                        "/api/chat/runs/cancel", {"run_id": run_id}, token
                    )["cancelled"]
                    if not cancelled:
                        raise RuntimeError(f"chat run was not cancellable: {run_id}")
                    return run_id
                event = ""
                data = ""
    raise RuntimeError("chat stream did not publish a run id")


def main() -> None:
    suffix = uuid.uuid4().hex[:10]
    password = "E2e-password-123!"
    auth = request(
        "/api/auth/register",
        {
            "username": f"e2e_{suffix}",
            "password": password,
            "confirm_password": password,
            "nickname": "E2E",
            "email": f"e2e_{suffix}@example.com",
        },
    )
    token = auth["token"]
    subject = request(
        "/api/subjects/create",
        {"name": f"E2E {suffix}", "description": "自动化端到端测试"},
        token,
    )["subject"]
    document = upload(token, subject["id"])["document"]

    deadline = time.monotonic() + 90
    status = document["status"]
    while time.monotonic() < deadline:
        detail = request("/api/documents/detail", {"id": document["id"]}, token)
        status = detail["document"]["status"]
        if status == "indexed":
            break
        if status in {"failed", "delete_failed"}:
            raise RuntimeError(f"document indexing failed: {detail['document']['error_message']}")
        time.sleep(1)
    else:
        raise RuntimeError(f"document was not indexed before timeout; last status={status}")

    expected_task_types = {"document.parse", "document.chunk", "document.embedding"}
    task_states = {task["task_type"]: task["status"] for task in detail.get("tasks", [])}
    if set(task_states) != expected_task_types or any(
        task_status != "success" for task_status in task_states.values()
    ):
        raise RuntimeError(f"document task chain is not closed: {task_states}")

    answer = request(
        "/api/chat/ask",
        {
            "subject_id": subject["id"],
            "query": "系统代号是什么？",
            "llm_provider": "openrouter",
            "llm_model": "openai/gpt-5.6-sol",
        },
        token,
    )
    if "Project Atlas" not in answer.get("answer", ""):
        raise RuntimeError(f"unexpected answer: {answer}")
    if not answer.get("chunks"):
        raise RuntimeError(f"answer has no knowledge citation: {answer}")
    if not answer.get("agent_steps"):
        raise RuntimeError(f"answer has no execution metadata: {answer}")
    run_id = answer.get("run_id", "")
    if not run_id:
        raise RuntimeError(f"answer has no durable run id: {answer}")
    run_detail = request("/api/chat/runs/detail", {"run_id": run_id}, token)
    run = run_detail["run"]
    if run["status"] != "completed":
        raise RuntimeError(f"chat run is not completed: {run}")
    if "Project Atlas" not in run_detail.get("result", {}).get("answer", ""):
        raise RuntimeError(f"chat run has no recoverable final result: {run_detail}")
    events = request(
        "/api/chat/runs/events",
        {"run_id": run_id, "after_sequence": 0, "limit": 200},
        token,
    )["list"]
    event_types = {event["type"] for event in events}
    if not {"run.running", "run.completed"}.issubset(event_types):
        raise RuntimeError(f"chat run event history is incomplete: {event_types}")

    cancelled_run_id = start_stream_and_cancel(token, subject["id"])
    cancel_deadline = time.monotonic() + 10
    while time.monotonic() < cancel_deadline:
        cancelled_run = request(
            "/api/chat/runs/detail", {"run_id": cancelled_run_id}, token
        )["run"]
        if cancelled_run["status"] == "cancelled":
            break
        time.sleep(0.2)
    else:
        raise RuntimeError(f"chat run cancellation did not become terminal: {cancelled_run}")

    resumed = request(
        "/api/chat/runs/resume", {"run_id": cancelled_run_id}, token
    )
    if resumed.get("run_id") != cancelled_run_id or "Project Atlas" not in resumed.get(
        "answer", ""
    ):
        raise RuntimeError(f"cancelled run did not resume successfully: {resumed}")

    try:
        request(
            "/api/chat/ask",
            {
                "subject_id": subject["id"],
                "query": "第四次提问以验证分布式限流",
                "llm_provider": "openrouter",
                "llm_model": "openai/gpt-5.6-sol",
            },
            token,
        )
    except RuntimeError as exc:
        if "HTTP 429" not in str(exc):
            raise
    else:
        raise RuntimeError("Redis rate limiter did not reject the request above the configured quota")

    print("端到端测试通过：上传、JetStream 索引、检索、Agent 编排、引用和 Redis 限流链路均正常。")


if __name__ == "__main__":
    main()
