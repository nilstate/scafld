from scafld.session_store import session_summary_payload


def test_human_override_round_does_not_double_count_blocked_challenge():
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


def test_human_override_without_verdict_still_counts_as_blocked_challenge():
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
