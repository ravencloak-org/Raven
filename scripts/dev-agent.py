#!/usr/bin/env python3
"""
Raven dev agent — Claude + Bash tool for Python, Go, and TypeScript tasks.

Usage:
    python scripts/dev-agent.py "write a new embedding provider for Mistral and test it"
    python scripts/dev-agent.py "run all Go tests and fix any failures"
    python scripts/dev-agent.py "add a vitest unit test for the chat store"

Requires: ANTHROPIC_API_KEY in environment.
"""

import os
import subprocess
import sys
import textwrap

import anthropic

REPO_ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
AI_WORKER_ROOT = os.path.join(REPO_ROOT, "ai-worker")
VENV_PYTHON = os.path.join(AI_WORKER_ROOT, ".venv", "bin", "python")

MODEL = "claude-opus-4-6"

SYSTEM_PROMPT = textwrap.dedent(f"""
    You are a development assistant for the Raven project — a multi-tenant AI knowledge base platform.

    Repository layout:
    - {REPO_ROOT}/api/          Go API server (Gin, JWT, SSE)
    - {REPO_ROOT}/ai-worker/    Python AI worker (gRPC, RAG, embeddings)
    - {REPO_ROOT}/frontend/     Vue.js 3 + Tailwind admin dashboard
    - {REPO_ROOT}/proto/        Protobuf definitions

    Runtime hints:
    - Python venv: {VENV_PYTHON} (use this for python commands, not bare `python`)
    - Go: use `go` from PATH, cd into {REPO_ROOT} or a subdirectory as needed
    - Node/TypeScript: use `npm` / `npx tsx`, cd into {REPO_ROOT}/frontend

    When writing code, create files at the correct paths in the repo.
    Always run tests after writing code to verify correctness.
    When a command fails, read the error and fix the root cause before retrying.
""").strip()


def run_bash(command: str, timeout: int = 60) -> str:
    """Execute a shell command and return combined stdout+stderr."""
    try:
        result = subprocess.run(
            command,
            shell=True,
            capture_output=True,
            text=True,
            timeout=timeout,
            cwd=REPO_ROOT,
        )
        output = result.stdout
        if result.stderr:
            output += "\n[stderr]\n" + result.stderr
        return output.strip() or "(no output)"
    except subprocess.TimeoutExpired:
        return f"[timeout after {timeout}s]"
    except Exception as e:
        return f"[error: {e}]"


def run_agent(task: str) -> None:
    client = anthropic.Anthropic(api_key=os.environ["ANTHROPIC_API_KEY"])

    messages: list[dict] = [{"role": "user", "content": task}]
    tools = [{"type": "bash_20250124", "name": "bash"}]

    print(f"\n[dev-agent] Task: {task}\n{'─' * 60}")

    while True:
        response = client.messages.create(
            model=MODEL,
            max_tokens=8096,
            system=SYSTEM_PROMPT,
            tools=tools,
            messages=messages,
        )

        messages.append({"role": "assistant", "content": response.content})

        # Collect tool calls and results
        tool_results = []
        for block in response.content:
            if block.type == "text":
                print(block.text)
            elif block.type == "tool_use" and block.name == "bash":
                cmd = block.input.get("command", "")
                print(f"\n$ {cmd}")
                output = run_bash(cmd)
                print(output)
                tool_results.append({
                    "type": "tool_result",
                    "tool_use_id": block.id,
                    "content": output,
                })

        if response.stop_reason == "end_turn":
            break

        if tool_results:
            messages.append({"role": "user", "content": tool_results})

    print(f"\n{'─' * 60}\n[dev-agent] Done.")


if __name__ == "__main__":
    if len(sys.argv) < 2:
        print("Usage: python scripts/dev-agent.py \"<task>\"")
        sys.exit(1)

    if not os.environ.get("ANTHROPIC_API_KEY"):
        print("Error: ANTHROPIC_API_KEY environment variable not set.")
        sys.exit(1)

    task = " ".join(sys.argv[1:])
    run_agent(task)
