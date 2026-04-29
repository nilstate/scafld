from __future__ import annotations

from dataclasses import dataclass, field

from scafld.error_codes import ErrorCode


@dataclass
class ScafldError(Exception):
    """Structured command error that callers can render consistently."""

    message: str
    details: list[str] = field(default_factory=list)
    code: str = ErrorCode.COMMAND_FAILED
    next_action: str | None = None
    exit_code: int = 1

    def __str__(self):
        return self.message
