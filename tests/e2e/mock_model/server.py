from __future__ import annotations

import json
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from typing import Any


class Handler(BaseHTTPRequestHandler):
    def do_GET(self) -> None:
        if self.path == "/healthz":
            self._json({"status": "ok"})
            return
        self.send_error(404)

    def do_POST(self) -> None:
        payload = self._payload()
        if self.path == "/v1/embeddings":
            inputs = payload.get("input", [])
            if isinstance(inputs, str):
                inputs = [inputs]
            dimensions = int(payload.get("dimensions", 1536))
            vector = [1.0] + [0.0] * (dimensions - 1)
            self._json(
                {
                    "object": "list",
                    "data": [
                        {"object": "embedding", "index": index, "embedding": vector}
                        for index, _ in enumerate(inputs)
                    ],
                    "usage": {"prompt_tokens": len(inputs), "total_tokens": len(inputs)},
                }
            )
            return
        if self.path == "/v1/chat/completions":
            prompt = _prompt(payload)
            if "请求路由器" in prompt:
                content = json.dumps(
                    {
                        "route": "rag",
                        "search_query": "系统代号",
                        "topic": "系统代号",
                        "reason": "需要查询知识库中的明确事实",
                    },
                    ensure_ascii=False,
                )
            elif "将问题改写成" in prompt:
                content = "系统代号"
            elif "证据评估器" in prompt:
                content = json.dumps(
                    {
                        "decision": "sufficient",
                        "coverage_score": 1,
                        "authority_score": 1,
                        "conflict_detected": False,
                        "missing_aspects": [],
                        "rewrite_query": "",
                        "reason": "资料直接覆盖问题",
                    },
                    ensure_ascii=False,
                )
            elif "回答忠实度校验器" in prompt:
                content = json.dumps(
                    {
                        "supported": True,
                        "invalid_references": [],
                        "unsupported_claims": [],
                        "reason": "回答由引用资料支持",
                    },
                    ensure_ascii=False,
                )
            else:
                content = "该系统代号是 Project Atlas。[引用1]"
            self._json(
                {
                    "id": "e2e-completion",
                    "object": "chat.completion",
                    "model": payload.get("model", "e2e-model"),
                    "choices": [
                        {
                            "index": 0,
                            "finish_reason": "stop",
                            "message": {"role": "assistant", "content": content},
                        }
                    ],
                    "usage": {
                        "prompt_tokens": 10,
                        "completion_tokens": 8,
                        "total_tokens": 18,
                    },
                }
            )
            return
        self.send_error(404)

    def log_message(self, format: str, *args: Any) -> None:
        return

    def _payload(self) -> dict[str, Any]:
        length = int(self.headers.get("Content-Length", "0"))
        return json.loads(self.rfile.read(length) or b"{}")

    def _json(self, payload: dict[str, Any]) -> None:
        body = json.dumps(payload, ensure_ascii=False).encode()
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)


def _prompt(payload: dict[str, Any]) -> str:
    messages = payload.get("messages", [])
    return "\n".join(
        str(message.get("content", "")) for message in messages if isinstance(message, dict)
    )


if __name__ == "__main__":
    ThreadingHTTPServer(("0.0.0.0", 8080), Handler).serve_forever()
