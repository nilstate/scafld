import os
import sys


C_RESET = "\033[0m"
C_BOLD = "\033[1m"
C_DIM = "\033[2m"
C_RED = "\033[31m"
C_GREEN = "\033[32m"
C_YELLOW = "\033[33m"
C_BLUE = "\033[34m"
C_MAGENTA = "\033[35m"
C_CYAN = "\033[36m"

STATUS_COLORS = {
    "draft": C_DIM,
    "under_review": C_YELLOW,
    "approved": C_BLUE,
    "in_progress": C_CYAN,
    "completed": C_GREEN,
    "failed": C_RED,
    "cancelled": C_DIM,
}


def supports_color():
    """Check if terminal supports color."""
    if os.environ.get("NO_COLOR"):
        return False
    if not hasattr(sys.stdout, "isatty") or not sys.stdout.isatty():
        return False
    return True


USE_COLOR = supports_color()


def c(code, text):
    """Colorize text if terminal supports it."""
    if not USE_COLOR:
        return text
    return f"{code}{text}{C_RESET}"
