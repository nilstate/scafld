import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

from scafld.session_store import (
    append_entry,
    default_session,
    load_session,
    record_phase_summary,
    session_summary_payload,
    set_phase_block,
    write_session,
)


def _check_human_override_round_does_not_double_count_blocked_challenge():
    summary = session_summary_payload(
        {
            "attempts": [],
            "entries": [
                {
                    "type": "challenge_verdict",
                    "gate": "review",
                    "review_round": 1,
                    "blocked": True,
                },
                {
                    "type": "human_override",
                    "gate": "review",
                    "review_round": 2,
                },
            ],
        }
    )

    assert summary["challenge_overrides"] == 1
    assert summary["challenge_blocked"] == 1
    assert summary["challenge_override_rate"] == 1.0


def _check_human_override_without_verdict_still_counts_as_blocked_challenge():
    summary = session_summary_payload(
        {
            "attempts": [],
            "entries": [
                {
                    "type": "human_override",
                    "gate": "review",
                    "review_round": 2,
                },
            ],
        }
    )

    assert summary["challenge_overrides"] == 1
    assert summary["challenge_blocked"] == 1
    assert summary["challenge_override_rate"] == 1.0


def _check_append_entry_appends_by_default_and_replacement_preserves_order():
    session = {"entries": []}

    append_entry(session, "attempt", criterion_id="ac1", status="fail", recorded_at="2026-04-30T00:00:00Z")
    append_entry(session, "attempt", criterion_id="ac1", status="pass", recorded_at="2026-04-30T00:00:01Z")
    append_entry(
        session,
        "provider_invocation",
        replace_keys={"invocation_id": "inv-1"},
        invocation_id="inv-1",
        status="running",
        recorded_at="2026-04-30T00:00:02Z",
    )
    append_entry(
        session,
        "provider_invocation",
        replace_keys={"invocation_id": "inv-1"},
        invocation_id="inv-1",
        status="completed",
        recorded_at="2026-04-30T00:00:03Z",
    )

    assert [entry["type"] for entry in session["entries"]] == ["attempt", "attempt", "provider_invocation"]
    assert [entry["status"] for entry in session["entries"]] == ["fail", "pass", "completed"]


def _check_write_session_is_atomic_on_replace_failure():
    with tempfile.TemporaryDirectory() as temp_dir:
        root = Path(temp_dir)
        session = default_session("atomic-task", model_profile="default")
        write_session(root, "atomic-task", session)
        path = root / ".scafld" / "runs" / "atomic-task" / "session.json"
        before = path.read_text(encoding="utf-8")

        broken = default_session("atomic-task", model_profile="default")
        broken["entries"].append({"type": "attempt", "recorded_at": "2026-04-30T00:00:00Z"})
        with patch("scafld.session_store.os.replace", side_effect=OSError("simulated replace failure")):
            try:
                write_session(root, "atomic-task", broken)
            except OSError:
                pass
            else:
                raise AssertionError("write_session should surface replace failures")

        assert path.read_text(encoding="utf-8") == before
        assert list(path.parent.glob(".session.json.*.tmp")) == []


def _check_phase_projection_state_is_written_to_session_phase_blocks():
    with tempfile.TemporaryDirectory() as temp_dir:
        root = Path(temp_dir)
        session = default_session("phase-task", model_profile="default")
        write_session(root, "phase-task", session)

        blocked = set_phase_block(root, "phase-task", "phase1", reason="needs evidence")
        assert blocked["phase_blocks"]["phase1"]["status"] == "blocked"
        assert blocked["phase_blocks"]["phase1"]["reason"] == "needs evidence"

        completed = record_phase_summary(root, "phase-task", "phase1", "done")
        assert completed["phase_blocks"]["phase1"]["status"] == "completed"
        assert completed["phase_blocks"]["phase1"]["reason"] is None

        loaded = load_session(root, "phase-task")
        assert loaded["phase_blocks"]["phase1"]["status"] == "completed"


class SessionStoreTests(unittest.TestCase):
    def test_human_override_round_does_not_double_count_blocked_challenge(self):
        _check_human_override_round_does_not_double_count_blocked_challenge()

    def test_human_override_without_verdict_still_counts_as_blocked_challenge(self):
        _check_human_override_without_verdict_still_counts_as_blocked_challenge()

    def test_append_entry_appends_by_default_and_replacement_preserves_order(self):
        _check_append_entry_appends_by_default_and_replacement_preserves_order()

    def test_write_session_is_atomic_on_replace_failure(self):
        _check_write_session_is_atomic_on_replace_failure()

    def test_phase_projection_state_is_written_to_session_phase_blocks(self):
        _check_phase_projection_state_is_written_to_session_phase_blocks()


if __name__ == "__main__":
    unittest.main()
