"""File-based memory store for per-session Claude memory tool integration.

Each org+session pair gets an isolated subdirectory. All path operations are
sandboxed to prevent directory traversal. Claude interacts with this via the
MEMORY_TOOL client-side tool definition.
"""

from __future__ import annotations

from pathlib import Path

import structlog

logger = structlog.get_logger(__name__)

# Client-side tool definition passed to the Anthropic API.
# Claude uses this to view, create, edit, and delete memory files.
MEMORY_TOOL: dict = {
    "name": "memory",
    "description": (
        "Read and write files in your persistent memory directory to maintain context "
        "across conversations. Use 'view' on /memories to see what's stored, then read "
        "specific files. After answering, save important context (user preferences, "
        "topics discussed, useful sources) so future sessions can pick up where you left off."
    ),
    "input_schema": {
        "type": "object",
        "properties": {
            "command": {
                "type": "string",
                "enum": ["view", "create", "str_replace", "delete"],
                "description": (
                    "view: list a directory or read a file. "
                    "create: write a new file. "
                    "str_replace: replace an exact string in an existing file. "
                    "delete: remove a file."
                ),
            },
            "path": {
                "type": "string",
                "description": "Path relative to /memories/ (e.g. 'session.md' or just '/memories').",
            },
            "file_text": {
                "type": "string",
                "description": "Full content to write (required for 'create').",
            },
            "old_str": {
                "type": "string",
                "description": "Exact text to find and replace (required for 'str_replace').",
            },
            "new_str": {
                "type": "string",
                "description": "Replacement text (required for 'str_replace').",
            },
        },
        "required": ["command", "path"],
    },
}


class MemoryStore:
    """File-based memory store scoped to a single org + session.

    Args:
        base_dir: Root directory for all memory files (e.g. ``/var/raven/memories``).
        org_id:   Tenant UUID — provides top-level isolation between organisations.
        session_id: Session or user UUID — isolates memory within an org.
    """

    def __init__(self, base_dir: str, org_id: str, session_id: str) -> None:
        self.root = Path(base_dir) / org_id / session_id
        self.root.mkdir(parents=True, exist_ok=True)
        logger.debug("memory_store_ready", root=str(self.root))

    # ------------------------------------------------------------------
    # Public dispatcher
    # ------------------------------------------------------------------

    def handle(self, command: str, path: str, **kwargs: str) -> str:
        """Execute a memory tool call and return the result string for Claude."""
        try:
            full_path = self._resolve(path)
        except ValueError as exc:
            return f"Error: {exc}"

        try:
            if command == "view":
                return self._view(full_path)
            if command == "create":
                return self._create(full_path, kwargs.get("file_text", ""))
            if command == "str_replace":
                return self._str_replace(
                    full_path,
                    kwargs.get("old_str", ""),
                    kwargs.get("new_str", ""),
                )
            if command == "delete":
                return self._delete(full_path)
            return f"Error: unknown command '{command}'"
        except Exception as exc:  # noqa: BLE001
            logger.warning("memory_op_error", command=command, path=path, error=str(exc))
            return f"Error: {exc}"

    # ------------------------------------------------------------------
    # Path resolution (prevents traversal)
    # ------------------------------------------------------------------

    def _resolve(self, path: str) -> Path:
        """Resolve *path* relative to ``self.root``, rejecting traversal attempts."""
        clean = path.lstrip("/")
        # Strip the logical /memories/ prefix Claude uses in its mental model
        for prefix in ("memories/", "memories"):
            if clean.startswith(prefix):
                clean = clean[len(prefix):]
                break

        resolved = (self.root / clean).resolve()
        try:
            resolved.relative_to(self.root.resolve())
        except ValueError:
            raise ValueError(f"Path '{path}' escapes the memory directory.")
        return resolved

    # ------------------------------------------------------------------
    # Individual operations
    # ------------------------------------------------------------------

    def _view(self, path: Path) -> str:
        if not path.exists():
            if path == self.root.resolve():
                return "(memory directory is empty — no files yet)"
            return f"The path {path.name!r} does not exist."
        if path.is_dir():
            entries = sorted(path.iterdir())
            if not entries:
                return "(empty directory)"
            return "\n".join(
                f"  {e.name}{'/' if e.is_dir() else ''}" for e in entries
            )
        lines = path.read_text(encoding="utf-8").splitlines()
        numbered = "\n".join(f"{i + 1:>6}\t{line}" for i, line in enumerate(lines))
        return f"Contents of {path.name!r} ({len(lines)} lines):\n{numbered}"

    def _create(self, path: Path, content: str) -> str:
        if path.exists():
            return (
                f"Error: {path.name!r} already exists. "
                "Use 'str_replace' to edit it or 'delete' first."
            )
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(content, encoding="utf-8")
        return f"File created successfully at: {path.name!r}"

    def _str_replace(self, path: Path, old_str: str, new_str: str) -> str:
        if not path.exists():
            return f"Error: {path.name!r} does not exist."
        if not old_str:
            return "Error: 'old_str' must not be empty."
        content = path.read_text(encoding="utf-8")
        if old_str not in content:
            return f"No replacement performed — {old_str!r} not found verbatim in {path.name!r}."
        if content.count(old_str) > 1:
            return f"Error: {old_str!r} appears more than once. Provide more surrounding context."
        path.write_text(content.replace(old_str, new_str, 1), encoding="utf-8")
        return f"Memory file {path.name!r} updated."

    def _delete(self, path: Path) -> str:
        if not path.exists():
            return f"Error: {path.name!r} does not exist."
        if path.is_dir():
            return "Error: cannot delete directories."
        path.unlink()
        return f"File {path.name!r} deleted."
