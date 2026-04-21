from dataclasses import dataclass, field


@dataclass
class ScafldError(Exception):
    """Structured command error that callers can render consistently."""

    message: str
    details: list[str] = field(default_factory=list)
    code: str = "command_failed"
    next_action: str | None = None
    exit_code: int = 1

    def __str__(self):
        return self.message
