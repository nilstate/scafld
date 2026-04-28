import json
import sys
import threading
import time
import unittest
import unittest.mock
from pathlib import Path
from unittest.mock import MagicMock

from scafld.errors import ScafldError
from scafld.error_codes import ErrorCode as EC
from scafld.review_runner import (
    WATCHDOG_GRACE_SECONDS,
    _auto_provider_candidates,
    _extract_claude_stdout,
    _is_model_rejection,
    _is_transient_provider_error,
    _model_from_model_usage,
    _model_hint_from_text,
    _provider_models,
    _provider_selection_reason,
    _provider_watchdog,
    _resolve_review_timeouts,
    _run_provider_subprocess,
    _valid_model_id,
    current_agent_provider,
    is_review_cancelled,
)


def _activity_now():
    return {"last_wall": time.time(), "last_mono": time.monotonic()}


def _empty_kill_state():
    return {"reason": None, "wall_elapsed": 0.0, "idle_age": 0.0}


class CurrentAgentProviderTest(unittest.TestCase):
    def test_explicit_override_wins(self):
        self.assertEqual(current_agent_provider({"SCAFLD_CURRENT_AGENT_PROVIDER": "claude"}), "claude")
        self.assertEqual(current_agent_provider({"SCAFLD_CURRENT_AGENT_PROVIDER": "codex"}), "codex")
        self.assertEqual(current_agent_provider({"SCAFLD_CURRENT_AGENT_PROVIDER": "unknown"}), "")

    def test_codex_inferred_from_env_prefix(self):
        self.assertEqual(current_agent_provider({"CODEX_HOME": "/tmp/codex"}), "codex")

    def test_claude_inferred_from_env_prefix(self):
        self.assertEqual(current_agent_provider({"CLAUDECODE_VERSION": "1"}), "claude")
        self.assertEqual(current_agent_provider({"CLAUDE_CODE_RUNTIME": "x"}), "claude")

    def test_no_signal_returns_empty(self):
        self.assertEqual(current_agent_provider({"PATH": "/usr/bin"}), "")

    def test_invalid_override_falls_through_to_inference(self):
        env = {"SCAFLD_CURRENT_AGENT_PROVIDER": "gemini", "CODEX_HOME": "/tmp"}
        self.assertEqual(current_agent_provider(env), "codex")


class AutoProviderCandidatesTest(unittest.TestCase):
    def test_default_prefers_codex(self):
        self.assertEqual(_auto_provider_candidates("warn", env={}), ("codex", "claude"))

    def test_codex_agent_prefers_claude(self):
        env = {"CODEX_HOME": "/tmp"}
        self.assertEqual(_auto_provider_candidates("warn", env=env), ("claude", "codex"))

    def test_disable_policy_locks_codex_only(self):
        env = {"CODEX_HOME": "/tmp"}
        self.assertEqual(_auto_provider_candidates("disable", env=env), ("codex",))

    def test_claude_agent_does_not_swap(self):
        env = {"CLAUDECODE_VERSION": "1"}
        self.assertEqual(_auto_provider_candidates("warn", env=env), ("codex", "claude"))


class ProviderSelectionReasonTest(unittest.TestCase):
    def test_avoid_codex_self_review(self):
        env = {"CODEX_HOME": "/tmp"}
        self.assertEqual(
            _provider_selection_reason("auto", "claude", "warn", env=env),
            "avoid_codex_self_review",
        )

    def test_no_alternate_provider_when_codex_agent_uses_codex(self):
        env = {"CODEX_HOME": "/tmp"}
        self.assertEqual(
            _provider_selection_reason("auto", "codex", "warn", env=env),
            "no_alternate_provider",
        )

    def test_no_alternate_skips_when_disable_policy(self):
        env = {"CODEX_HOME": "/tmp"}
        self.assertEqual(
            _provider_selection_reason("auto", "codex", "disable", env=env),
            "",
        )

    def test_codex_unavailable_when_claude_chosen_outside_codex_agent(self):
        self.assertEqual(
            _provider_selection_reason("auto", "claude", "warn", env={}),
            "codex_unavailable",
        )

    def test_explicit_provider_request_returns_empty(self):
        self.assertEqual(_provider_selection_reason("codex", "codex", "warn", env={}), "")
        self.assertEqual(_provider_selection_reason("claude", "claude", "warn", env={}), "")


class ValidModelIdTest(unittest.TestCase):
    def test_accepts_well_formed_ids(self):
        self.assertEqual(_valid_model_id("gpt-5.5"), "gpt-5.5")
        self.assertEqual(_valid_model_id("claude-opus-4-7"), "claude-opus-4-7")
        self.assertEqual(_valid_model_id("o3-mini"), "o3-mini")

    def test_rejects_empty_and_whitespace(self):
        self.assertEqual(_valid_model_id(""), "")
        self.assertEqual(_valid_model_id("   "), "")
        self.assertEqual(_valid_model_id(None), "")

    def test_rejects_disallowed_characters(self):
        self.assertEqual(_valid_model_id("gpt 5.5"), "")
        self.assertEqual(_valid_model_id("gpt;5"), "")
        self.assertEqual(_valid_model_id("gpt-5.5\n"), "gpt-5.5")  # documents that .strip() is applied

    def test_rejects_oversize(self):
        oversize = "a" * 200
        self.assertEqual(_valid_model_id(oversize), "")

    def test_known_prefix_filter(self):
        self.assertEqual(_valid_model_id("gpt-5.5", require_known_prefix=True), "gpt-5.5")
        self.assertEqual(_valid_model_id("claude-opus-4-7", require_known_prefix=True), "claude-opus-4-7")
        self.assertEqual(_valid_model_id("totally-made-up-1", require_known_prefix=True), "")

    def test_known_prefix_does_not_block_unprefixed_when_disabled(self):
        self.assertEqual(_valid_model_id("totally-made-up-1"), "totally-made-up-1")


class ModelHintFromTextTest(unittest.TestCase):
    def test_extracts_simple_hint(self):
        model, source = _model_hint_from_text("model: gpt-5.5\n")
        self.assertEqual(model, "gpt-5.5")
        self.assertEqual(source, "inferred")

    def test_extracts_quoted_hint(self):
        model, source = _model_hint_from_text('model: "claude-opus-4-7"')
        self.assertEqual(model, "claude-opus-4-7")
        self.assertEqual(source, "inferred")

    def test_extracts_alternate_separator_and_label(self):
        model, _ = _model_hint_from_text("model_id=gpt-5.5;")
        self.assertEqual(model, "gpt-5.5")

    def test_returns_empty_when_no_hint(self):
        self.assertEqual(_model_hint_from_text("nothing here"), ("", ""))
        self.assertEqual(_model_hint_from_text(""), ("", ""))

    def test_rejects_implausible_prefix(self):
        self.assertEqual(_model_hint_from_text("model: totally-made-up-1"), ("", ""))


class ModelFromModelUsageTest(unittest.TestCase):
    def test_single_entry_returns_key(self):
        self.assertEqual(
            _model_from_model_usage({"modelUsage": {"claude-opus-4-7": {"costUSD": 0.12}}}),
            "claude-opus-4-7",
        )

    def test_multiple_entries_returns_highest_cost(self):
        usage = {
            "modelUsage": {
                "claude-opus-4-7": {"costUSD": 0.50},
                "claude-haiku-4-5": {"costUSD": 0.05},
            }
        }
        self.assertEqual(_model_from_model_usage(usage), "claude-opus-4-7")

    def test_zero_costs_falls_back_to_first_sorted(self):
        usage = {
            "modelUsage": {
                "claude-haiku-4-5": {"costUSD": 0},
                "claude-opus-4-7": {"costUSD": 0},
            }
        }
        # sort order is alphabetical, first valid wins when no cost separates them
        self.assertEqual(_model_from_model_usage(usage), "claude-haiku-4-5")

    def test_non_finite_cost_treated_as_zero(self):
        usage = {
            "modelUsage": {
                "claude-opus-4-7": {"costUSD": float("inf")},
                "claude-haiku-4-5": {"costUSD": 0.10},
            }
        }
        self.assertEqual(_model_from_model_usage(usage), "claude-haiku-4-5")

    def test_missing_or_invalid_payload_returns_empty(self):
        self.assertEqual(_model_from_model_usage(None), "")
        self.assertEqual(_model_from_model_usage({}), "")
        self.assertEqual(_model_from_model_usage({"modelUsage": {}}), "")
        self.assertEqual(_model_from_model_usage({"modelUsage": "not a dict"}), "")


class ExtractClaudeStdoutTest(unittest.TestCase):
    def test_plain_text_passes_through(self):
        raw, model, source, session = _extract_claude_stdout("plain output\n")
        self.assertEqual(raw, "plain output\n")
        self.assertEqual(model, "")
        self.assertEqual(source, "")
        self.assertEqual(session, "")

    def test_text_with_model_hint_extracts_inferred_model(self):
        raw, model, source, session = _extract_claude_stdout("review verdict pass\nmodel: claude-opus-4-7\n")
        self.assertEqual(model, "claude-opus-4-7")
        self.assertEqual(source, "inferred")
        self.assertEqual(session, "")
        self.assertIn("review verdict pass", raw)

    def test_json_with_top_level_model_observed(self):
        payload = json.dumps({
            "model": "claude-opus-4-7",
            "session_id": "11111111-1111-4111-8111-111111111111",
            "result": "{\"verdict\": \"pass\"}",
        })
        raw, model, source, session = _extract_claude_stdout(payload)
        self.assertEqual(model, "claude-opus-4-7")
        self.assertEqual(source, "observed")
        self.assertEqual(session, "11111111-1111-4111-8111-111111111111")
        self.assertEqual(raw, "{\"verdict\": \"pass\"}")

    def test_json_with_only_model_usage(self):
        payload = json.dumps({
            "result": "{\"verdict\": \"pass\"}",
            "modelUsage": {"claude-opus-4-7": {"costUSD": 0.10}},
        })
        raw, model, source, session = _extract_claude_stdout(payload)
        self.assertEqual(model, "claude-opus-4-7")
        self.assertEqual(source, "observed")
        self.assertEqual(session, "")
        self.assertEqual(raw, "{\"verdict\": \"pass\"}")

    def test_malformed_json_falls_back_to_raw_text(self):
        bad = '{"model": "claude-opus-4-7", "result": "incomplete'
        raw, model, source, session = _extract_claude_stdout(bad)
        # JSON-style `"model":` does not match the unstructured hint regex
        # (the regex requires `model:` or `model=`, not `"model":`). The
        # function must still return without raising and surface raw stdout.
        self.assertEqual(raw, bad)
        self.assertEqual(model, "")
        self.assertEqual(source, "")
        self.assertEqual(session, "")

    def test_malformed_json_with_unstructured_hint_extracts_model(self):
        bad = "model: claude-opus-4-7\npartial output then {invalid"
        raw, model, source, session = _extract_claude_stdout(bad)
        self.assertEqual(raw, bad)
        self.assertEqual(model, "claude-opus-4-7")
        self.assertEqual(source, "inferred")
        self.assertEqual(session, "")

    def test_non_dict_json_falls_back_to_text(self):
        payload = json.dumps(["not", "a", "dict"])
        raw, model, source, session = _extract_claude_stdout(payload)
        self.assertEqual(raw, payload)
        self.assertEqual(model, "")
        self.assertEqual(source, "")
        self.assertEqual(session, "")


class ExtractClaudeStdoutNDJSONTest(unittest.TestCase):
    """`--output-format stream-json --verbose` emits NDJSON. Parser walks
    each line, pulls session/model from system.init, and pulls the
    structured packet (or free-text result) from the final result event."""

    def _ndjson(self, *events):
        return "\n".join(json.dumps(e) for e in events) + "\n"

    def test_happy_path_with_structured_output(self):
        stream = self._ndjson(
            {
                "type": "system",
                "subtype": "init",
                "model": "claude-opus-4-7",
                "session_id": "55191b22-f249-441f-9aa5-031b3a53045c",
            },
            {"type": "assistant", "message": {"content": [{"type": "text", "text": "thinking..."}]}},
            {
                "type": "result",
                "subtype": "success",
                "result": "free-text summary",
                "structured_output": {"verdict": "pass", "review_summary": "ok"},
            },
        )
        raw, model, source, session = _extract_claude_stdout(stream)
        self.assertEqual(json.loads(raw), {"verdict": "pass", "review_summary": "ok"})
        self.assertEqual(model, "claude-opus-4-7")
        self.assertEqual(source, "observed")
        self.assertEqual(session, "55191b22-f249-441f-9aa5-031b3a53045c")

    def test_falls_back_to_free_text_result_when_no_structured_output(self):
        stream = self._ndjson(
            {"type": "system", "subtype": "init", "model": "claude-opus-4-7", "session_id": "abc"},
            {"type": "result", "subtype": "success", "result": "verdict=pass"},
        )
        raw, model, _source, session = _extract_claude_stdout(stream)
        self.assertEqual(raw, "verdict=pass")
        self.assertEqual(model, "claude-opus-4-7")
        self.assertEqual(session, "abc")

    def test_truncated_stream_returns_stdout_as_raw(self):
        # Init present, no result event (process killed mid-run). Parser
        # should not raise; raw_output falls back to stdout so the
        # diagnostic still has a trail.
        stream = self._ndjson(
            {"type": "system", "subtype": "init", "model": "claude-opus-4-7", "session_id": "abc"},
            {"type": "assistant", "message": {"content": [{"type": "text", "text": "in progress"}]}},
        )
        raw, model, _source, session = _extract_claude_stdout(stream)
        self.assertEqual(raw, stream)
        self.assertEqual(model, "claude-opus-4-7")
        self.assertEqual(session, "abc")

    def test_garbage_lines_tolerated(self):
        # Stray non-JSON line in the middle (e.g. a stderr leak or shell
        # warning). Parser must keep walking and still find the result.
        stream = (
            json.dumps({"type": "system", "subtype": "init", "model": "claude-opus-4-7", "session_id": "abc"}) + "\n"
            "warning: rate limit close\n"  # not JSON
            + json.dumps({"type": "result", "subtype": "success", "result": "ok"}) + "\n"
        )
        raw, model, _source, session = _extract_claude_stdout(stream)
        self.assertEqual(raw, "ok")
        self.assertEqual(model, "claude-opus-4-7")
        self.assertEqual(session, "abc")

    def test_leading_noise_routes_to_ndjson_parser(self):
        # Claude or a wrapper may write a banner or warning to stdout
        # before the stream-json events start. The detection heuristic
        # must scan past leading non-JSON lines and route to the NDJSON
        # parser when typed events appear later, not fall back to the
        # legacy single-blob parser (which would return raw stdout with
        # no extracted packet).
        stream = (
            "warning: starting up\n"
            + json.dumps({"type": "system", "subtype": "init", "model": "claude-opus-4-7", "session_id": "abc"}) + "\n"
            + json.dumps({
                "type": "result",
                "subtype": "success",
                "result": "ok",
                "structured_output": {"verdict": "pass"},
            }) + "\n"
        )
        raw, model, source, session = _extract_claude_stdout(stream)
        self.assertEqual(json.loads(raw), {"verdict": "pass"})
        self.assertEqual(model, "claude-opus-4-7")
        self.assertEqual(source, "observed")
        self.assertEqual(session, "abc")

    def test_legacy_single_blob_still_parses(self):
        # Safety: if claude regresses to single-blob JSON or scafld talks
        # to an older binary, the parser must still extract the packet.
        # The detection heuristic only routes to NDJSON when at least one
        # line parses as a typed event; a single blob without `\n` between
        # objects looks like one big JSON object → legacy path.
        payload = json.dumps({
            "model": "claude-opus-4-7",
            "session_id": "11111111-1111-4111-8111-111111111111",
            "result": "{\"verdict\": \"pass\"}",
        })
        raw, model, source, session = _extract_claude_stdout(payload)
        self.assertEqual(raw, "{\"verdict\": \"pass\"}")
        self.assertEqual(model, "claude-opus-4-7")
        self.assertEqual(source, "observed")
        self.assertEqual(session, "11111111-1111-4111-8111-111111111111")

    def test_pulls_model_from_result_modelusage_when_init_lacks_it(self):
        # Older claude builds may omit `model` from system.init; the result
        # event's modelUsage should still surface the observed model.
        stream = self._ndjson(
            {"type": "system", "subtype": "init", "session_id": "abc"},
            {
                "type": "result",
                "subtype": "success",
                "result": "ok",
                "modelUsage": {"claude-opus-4-7": {"costUSD": 0.05}},
            },
        )
        _raw, model, source, _session = _extract_claude_stdout(stream)
        self.assertEqual(model, "claude-opus-4-7")
        self.assertEqual(source, "observed")


class ClaudeNDJSONEventInspectorTest(unittest.TestCase):
    """The pump's event_inspector is called with each line as it arrives so
    the diagnostic can show event counts in flight without re-scanning the
    buffer at kill time."""

    def test_returns_type_with_subtype_when_present(self):
        from scafld.review_runner import _claude_ndjson_event_inspector
        line = json.dumps({"type": "system", "subtype": "init", "model": "x"})
        self.assertEqual(_claude_ndjson_event_inspector(line), "system.init")

    def test_returns_type_only_when_no_subtype(self):
        from scafld.review_runner import _claude_ndjson_event_inspector
        line = json.dumps({"type": "assistant", "message": {}})
        self.assertEqual(_claude_ndjson_event_inspector(line), "assistant")

    def test_returns_none_on_parse_failure(self):
        from scafld.review_runner import _claude_ndjson_event_inspector
        self.assertIsNone(_claude_ndjson_event_inspector("not json"))

    def test_returns_none_on_non_dict(self):
        from scafld.review_runner import _claude_ndjson_event_inspector
        self.assertIsNone(_claude_ndjson_event_inspector(json.dumps([1, 2, 3])))

    def test_returns_none_on_missing_type(self):
        from scafld.review_runner import _claude_ndjson_event_inspector
        self.assertIsNone(_claude_ndjson_event_inspector(json.dumps({"foo": "bar"})))


class StreamPumpEventInspectorTest(unittest.TestCase):
    """End-to-end check that the inspector is wired into the pump and the
    event summary shows up in the ProviderProcessResult."""

    REPO_ROOT = Path(__file__).resolve().parent.parent

    def test_pump_counts_event_types_via_inspector(self):
        # Spawn python that emits three NDJSON events to stdout; verify the
        # event_summary on the ProviderProcessResult tallies them.
        events = [
            json.dumps({"type": "system", "subtype": "init", "model": "x"}),
            json.dumps({"type": "assistant", "message": {"content": []}}),
            json.dumps({"type": "result", "subtype": "success", "result": "ok"}),
        ]
        nl_events = "\\n".join(events)
        code = (
            "import sys, time\n"
            "events = [\n"
            f"    {events[0]!r},\n"
            f"    {events[1]!r},\n"
            f"    {events[2]!r},\n"
            "]\n"
            "for e in events:\n"
            "    sys.stdout.write(e + '\\n')\n"
            "    sys.stdout.flush()\n"
            "    time.sleep(0.02)\n"
        )
        from scafld.review_runner import _claude_ndjson_event_inspector
        result = _run_provider_subprocess(
            [sys.executable, "-c", code],
            prompt="",
            root=self.REPO_ROOT,
            provider="claude",
            idle_timeout_seconds=5,
            absolute_max_seconds=5,
            stdout_event_inspector=_claude_ndjson_event_inspector,
        )
        self.assertEqual(result.returncode, 0)
        self.assertEqual(result.stdout_event_summary.get("system.init"), 1)
        self.assertEqual(result.stdout_event_summary.get("assistant"), 1)
        self.assertEqual(result.stdout_event_summary.get("result.success"), 1)


class IsModelRejectionTest(unittest.TestCase):
    def test_clean_exit_never_classifies(self):
        self.assertEqual(_is_model_rejection(0, "model not found", ""), "")

    def test_unknown_model_in_stderr(self):
        self.assertEqual(
            _is_model_rejection(1, "", "error: unknown model 'gpt-5.5'"),
            "unknown model",
        )

    def test_model_not_found_case_insensitive(self):
        self.assertEqual(
            _is_model_rejection(1, "Model Not Found in account", ""),
            "model not found",
        )

    def test_does_not_have_access(self):
        self.assertEqual(
            _is_model_rejection(2, "", "your account does not have access"),
            "does not have access",
        )

    def test_not_authorized_to_use_this_model(self):
        self.assertEqual(
            _is_model_rejection(1, "", "you are not authorized to use this model"),
            "not authorized to use this model",
        )

    def test_unrelated_failure_returns_empty(self):
        self.assertEqual(_is_model_rejection(1, "", "connection refused"), "")
        self.assertEqual(_is_model_rejection(1, "rate limit exceeded", ""), "")


class ProviderModelsTest(unittest.TestCase):
    def test_default_when_unset(self):
        self.assertEqual(_provider_models({}, "codex"), ("gpt-5.5",))
        self.assertEqual(_provider_models({}, "claude"), ("claude-opus-4-7",))

    def test_single_string_returns_one_tuple(self):
        cfg = {"codex": {"model": "custom-codex-1"}}
        self.assertEqual(_provider_models(cfg, "codex"), ("custom-codex-1",))

    def test_list_preserves_order(self):
        cfg = {"claude": {"model": ["claude-opus-4-7", "claude-haiku-4-5"]}}
        self.assertEqual(
            _provider_models(cfg, "claude"),
            ("claude-opus-4-7", "claude-haiku-4-5"),
        )

    def test_empty_list_raises(self):
        with self.assertRaises(ValueError):
            _provider_models({"codex": {"model": []}}, "codex")

    def test_non_string_entry_raises(self):
        with self.assertRaises(ValueError):
            _provider_models({"codex": {"model": ["gpt-5.5", 123]}}, "codex")

    def test_blank_entry_raises(self):
        with self.assertRaises(ValueError):
            _provider_models({"codex": {"model": ["gpt-5.5", "  "]}}, "codex")

    def test_oversize_list_raises(self):
        cfg = {"codex": {"model": [f"m{i}" for i in range(9)]}}
        with self.assertRaises(ValueError):
            _provider_models(cfg, "codex")

    def test_auto_provider_returns_empty_tuple(self):
        self.assertEqual(_provider_models({}, "auto"), ())

    def test_non_dict_provider_entry_raises(self):
        with self.assertRaises(ValueError):
            _provider_models({"codex": "not-a-dict"}, "codex")


class IsTransientProviderErrorTest(unittest.TestCase):
    def test_clean_exit_never_classifies(self):
        self.assertEqual(_is_transient_provider_error(0, "stream idle timeout", ""), "")

    def test_stream_idle_timeout(self):
        self.assertEqual(
            _is_transient_provider_error(1, "API Error: Stream idle timeout - partial response received", ""),
            "stream idle timeout",
        )

    def test_rate_limit_in_stderr(self):
        self.assertEqual(
            _is_transient_provider_error(1, "", "429 Too Many Requests"),
            "429 too many requests",
        )

    def test_bare_429_in_unrelated_text_does_not_classify(self):
        # Tightened from a bare '429' substring to avoid false positives.
        self.assertEqual(
            _is_transient_provider_error(1, "", "exit code 4290 from helper"),
            "",
        )
        self.assertEqual(
            _is_transient_provider_error(1, "", "id=429 not found"),
            "",
        )

    def test_503_service_unavailable(self):
        self.assertEqual(
            _is_transient_provider_error(1, "", "503 Service Unavailable: try again later"),
            "503 service unavailable",
        )

    def test_502_bad_gateway(self):
        self.assertEqual(
            _is_transient_provider_error(1, "", "502 Bad Gateway"),
            "502 bad gateway",
        )

    def test_504_gateway_timeout(self):
        self.assertEqual(
            _is_transient_provider_error(1, "", "504 Gateway Timeout"),
            "504 gateway timeout",
        )

    def test_connection_reset(self):
        self.assertEqual(
            _is_transient_provider_error(2, "", "connection reset by peer"),
            "connection reset",
        )

    def test_temporarily_unavailable(self):
        self.assertEqual(
            _is_transient_provider_error(1, "service is temporarily unavailable", ""),
            "temporarily unavailable",
        )

    def test_model_rejection_does_not_classify_as_transient(self):
        # Model rejection is a separate category — must NOT also classify here.
        # The classifier doesn't know about model rejection per se, so it must
        # not match any of the rejection signatures by accident.
        self.assertEqual(
            _is_transient_provider_error(1, "", "error: unknown model 'gpt-5.5'"),
            "",
        )
        self.assertEqual(
            _is_transient_provider_error(1, "", "error: model not available on this account"),
            "",
        )

    def test_unrelated_failure_returns_empty(self):
        self.assertEqual(_is_transient_provider_error(1, "", "permission denied"), "")
        self.assertEqual(_is_transient_provider_error(1, "syntax error", ""), "")


class FakeClock:
    """Sleep-recording stand-in for the watchdog's `sleep` parameter.
    Lets tests assert the sleep durations the watchdog issued without
    the test taking real wall-clock time for the no-op cases.
    """

    def __init__(self):
        self.sleeps = []

    def sleep(self, seconds):
        self.sleeps.append(seconds)


def _make_proc(poll_returns):
    """Build a mock proc whose poll() walks through `poll_returns` in order,
    then sticks at the final value indefinitely. Tests that drive the
    watchdog with a real-time clock can call poll() many times during
    the grace window; sticking at the last value prevents StopIteration.
    """
    sequence = list(poll_returns)
    state = {"index": 0}

    def _poll():
        if state["index"] < len(sequence) - 1:
            value = sequence[state["index"]]
            state["index"] += 1
            return value
        return sequence[-1]

    proc = MagicMock()
    proc.poll.side_effect = _poll
    proc.pid = 12345
    return proc


class ProviderWatchdogTest(unittest.TestCase):
    def _watch(self, proc, done, *, absolute_max=60, idle_timeout=600, activity=None, kill_state=None, sleep=None):
        if activity is None:
            activity = _activity_now()
        if kill_state is None:
            kill_state = _empty_kill_state()
        _provider_watchdog(
            proc,
            idle_timeout_seconds=idle_timeout,
            absolute_max_seconds=absolute_max,
            activity=activity,
            done=done,
            kill_state=kill_state,
            sleep=sleep if sleep is not None else (lambda _s: None),
        )
        return kill_state

    def test_returns_immediately_when_done_set(self):
        proc = _make_proc([None])
        done = threading.Event()
        done.set()
        clock = FakeClock()
        with unittest.mock.patch("scafld.review_runner._terminate_provider_process_group") as term, \
             unittest.mock.patch("scafld.review_runner._kill_provider_process_group") as kill:
            self._watch(proc, done, sleep=clock.sleep)
        term.assert_not_called()
        kill.assert_not_called()
        self.assertEqual(clock.sleeps, [])

    def test_leaves_healthy_proc_alone_when_done_set_before_deadline(self):
        # Watchdog exits cleanly when `done` is signaled before the
        # deadline elapses (the normal "subprocess finished" path).
        proc = _make_proc([None, None, None, 0])
        done = threading.Event()
        clock = FakeClock()

        def _set_done_after_two_ticks(_seconds):
            clock.sleeps.append(_seconds)
            if len(clock.sleeps) >= 2:
                done.set()

        with unittest.mock.patch("scafld.review_runner._terminate_provider_process_group") as term, \
             unittest.mock.patch("scafld.review_runner._kill_provider_process_group") as kill:
            self._watch(proc, done, sleep=_set_done_after_two_ticks)
        term.assert_not_called()
        kill.assert_not_called()

    def test_terminates_then_kills_on_absolute_max_when_proc_never_exits(self):
        # Sequence: while waiting for the wall ceiling poll() not consulted;
        # once absolute_max fires, watchdog calls poll() to check liveness,
        # sees None, sends SIGTERM. Then loops up to grace polling poll() —
        # still None — sends SIGKILL. Idle threshold set high so absolute_max
        # is the trigger.
        proc = _make_proc([None, None, None, None, None, None])
        done = threading.Event()
        clock = FakeClock()
        with unittest.mock.patch("scafld.review_runner._terminate_provider_process_group") as term, \
             unittest.mock.patch("scafld.review_runner._kill_provider_process_group") as kill:
            kill_state = self._watch(proc, done, absolute_max=1.5, idle_timeout=600, sleep=clock.sleep)
        term.assert_called_once_with(proc)
        kill.assert_called_once_with(proc)
        self.assertEqual(kill_state["reason"], "absolute_max")

    def test_terminate_only_when_proc_exits_during_grace(self):
        # absolute_max fires, poll() returns None at trigger, then exits
        # during grace window.
        proc = _make_proc([None, None, None, 0])
        done = threading.Event()
        clock = FakeClock()
        with unittest.mock.patch("scafld.review_runner._terminate_provider_process_group") as term, \
             unittest.mock.patch("scafld.review_runner._kill_provider_process_group") as kill:
            self._watch(proc, done, absolute_max=1.0, idle_timeout=600, sleep=clock.sleep)
        term.assert_called_once_with(proc)
        kill.assert_not_called()

    def test_kills_on_idle_timeout(self):
        # Activity clock is stuck in the past; idle threshold short. Watchdog
        # should fire on idle_timeout, not absolute_max.
        proc = _make_proc([None, None, None, None, None, None])
        done = threading.Event()
        old_time = time.time() - 10
        old_mono = time.monotonic() - 10
        activity = {"last_wall": old_time, "last_mono": old_mono}
        with unittest.mock.patch("scafld.review_runner._terminate_provider_process_group") as term, \
             unittest.mock.patch("scafld.review_runner._kill_provider_process_group") as kill:
            kill_state = self._watch(
                proc,
                done,
                absolute_max=600,
                idle_timeout=1.0,
                activity=activity,
                sleep=lambda _s: None,
            )
        term.assert_called_once_with(proc)
        kill.assert_called_once_with(proc)
        self.assertEqual(kill_state["reason"], "idle_timeout")
        self.assertGreaterEqual(kill_state["idle_age"], 1.0)

    def test_idle_does_not_fire_when_activity_recent(self):
        # Activity clock kept fresh inside the sleep callback simulates a
        # productive run; watchdog should not fire on idle even though the
        # threshold is short, because every tick refreshes activity.
        proc = _make_proc([None, None, None, 0])
        done = threading.Event()
        activity = _activity_now()

        def _refresh_activity_and_set_done(_seconds):
            activity["last_wall"] = time.time()
            activity["last_mono"] = time.monotonic()
            if not done.is_set():
                done.set()

        with unittest.mock.patch("scafld.review_runner._terminate_provider_process_group") as term, \
             unittest.mock.patch("scafld.review_runner._kill_provider_process_group") as kill:
            kill_state = self._watch(
                proc,
                done,
                absolute_max=600,
                idle_timeout=0.1,
                activity=activity,
                sleep=_refresh_activity_and_set_done,
            )
        term.assert_not_called()
        kill.assert_not_called()
        self.assertIsNone(kill_state["reason"])


class RunExternalReviewTransientRetryTest(unittest.TestCase):
    """Cover the transient retry branching in `run_external_review` without
    spawning a real subprocess. Mocks the inner per-attempt function and
    asserts retry counts, signature recording, and exhaustion behavior."""

    def _make_resolved(self):
        from scafld.review_runner import ResolvedReviewRunner
        return ResolvedReviewRunner(
            runner="external",
            provider="codex",
            model="m1",
            models=("m1",),
            idle_timeout_seconds=30,
            absolute_max_seconds=30,
            fallback_policy="warn",
        )

    def _make_result(self):
        from scafld.review_runner import ExternalReviewResult
        return ExternalReviewResult(
            reviewer_mode="fresh_agent",
            reviewer_session="",
            reviewer_isolation="codex_read_only_ephemeral",
            pass_results={},
            sections={},
            blocking=[],
            non_blocking=[],
            verdict="pass",
            provenance={},
            raw_output="",
            packet=None,
        )

    def test_succeeds_on_first_attempt_records_no_retries(self):
        from scafld import review_runner
        result = self._make_result()
        with unittest.mock.patch.object(review_runner, "_run_external_review_with_model_fallback", return_value=result) as inner:
            out = review_runner.run_external_review(None, "task", "prompt", {}, self._make_resolved())
        inner.assert_called_once()
        self.assertNotIn("transient_retries", out.provenance)


    def test_retries_on_transient_then_succeeds(self):
        from scafld import review_runner
        from scafld.errors import ScafldError
        from scafld.error_codes import ErrorCode as EC

        first = ScafldError("external review runner failed via codex", code=EC.COMMAND_FAILED)
        first._transient_signature = "stream idle timeout"
        result = self._make_result()
        call_log = []

        def _side_effect(*a, **kw):
            call_log.append(1)
            if len(call_log) == 1:
                raise first
            return result

        with unittest.mock.patch.object(review_runner, "_run_external_review_with_model_fallback", side_effect=_side_effect), \
             unittest.mock.patch.object(review_runner.time, "sleep") as fake_sleep:
            out = review_runner.run_external_review(None, "task", "prompt", {}, self._make_resolved())
        self.assertEqual(len(call_log), 2)
        # The helper-bounded backoff exits on its first elapsed() check
        # (stub returns 10000s, exceeds the configured cap), so no sleeps
        # are issued. That's the correct semantic — the wait loop only
        # sleeps while elapsed() is below the budget.
        self.assertEqual(out.provenance.get("transient_retries"), 1)
        attempts = out.provenance.get("transient_attempts") or []
        self.assertEqual(len(attempts), 1)
        self.assertEqual(attempts[0]["signature"], "stream idle timeout")

    def test_exhausts_retries_then_raises(self):
        from scafld import review_runner
        from scafld.errors import ScafldError
        from scafld.error_codes import ErrorCode as EC

        def _raise(*a, **kw):
            err = ScafldError("external review runner failed via codex", code=EC.COMMAND_FAILED)
            err._transient_signature = "rate limit"
            raise err

        with unittest.mock.patch.object(review_runner, "_run_external_review_with_model_fallback", side_effect=_raise), \
             unittest.mock.patch.object(review_runner.time, "sleep"):
            with self.assertRaises(ScafldError) as ctx:
                review_runner.run_external_review(None, "task", "prompt", {}, self._make_resolved())
        self.assertIn("transient retry history", "\n".join(ctx.exception.details))

    def test_non_transient_failure_raises_without_retry(self):
        from scafld import review_runner
        from scafld.errors import ScafldError
        from scafld.error_codes import ErrorCode as EC

        err = ScafldError("external review runner failed via codex", code=EC.COMMAND_FAILED)
        # Note: no _transient_signature attribute set → non-retryable.

        with unittest.mock.patch.object(review_runner, "_run_external_review_with_model_fallback", side_effect=err) as inner, \
             unittest.mock.patch.object(review_runner.time, "sleep") as fake_sleep:
            with self.assertRaises(ScafldError):
                review_runner.run_external_review(None, "task", "prompt", {}, self._make_resolved())
        self.assertEqual(inner.call_count, 1)
        fake_sleep.assert_not_called()

    def test_multi_model_fallback_constructs_attempt_runner(self):
        # Mocks `_run_external_review_once` (the layer below the fallback
        # wrapper), so the multi-model branch actually runs the
        # constructor for each candidate model. Catches a regression
        # where the per-attempt ResolvedReviewRunner couldn't be built
        # at all (e.g. dataclass field rename without updating the call
        # site).
        from scafld import review_runner
        from scafld.review_runner import ResolvedReviewRunner
        from scafld.errors import ScafldError
        from scafld.error_codes import ErrorCode as EC

        resolved = ResolvedReviewRunner(
            runner="external",
            provider="codex",
            model="m1",
            models=("m1", "m2"),
            idle_timeout_seconds=30,
            absolute_max_seconds=30,
            fallback_policy="warn",
        )
        attempt_runners = []

        def _once(_root, _task, _prompt, _topology, attempt_runner, *, on_start=None):
            attempt_runners.append(attempt_runner)
            if attempt_runner.model == "m1":
                err = ScafldError("model unavailable", code=EC.COMMAND_FAILED)
                err._model_rejection_signature = "unknown model"
                raise err
            return self._make_result()

        with unittest.mock.patch.object(review_runner, "_run_external_review_once", side_effect=_once):
            out = review_runner.run_external_review(None, "task", "prompt", {}, resolved)

        self.assertEqual([r.model for r in attempt_runners], ["m1", "m2"])
        for runner in attempt_runners:
            # Per-attempt runner preserves transport knobs from the parent.
            self.assertEqual(runner.idle_timeout_seconds, 30)
            self.assertEqual(runner.absolute_max_seconds, 30)
            self.assertEqual(runner.fallback_policy, "warn")
        self.assertEqual(out.verdict, "pass")


class IsReviewCancelledTest(unittest.TestCase):
    def test_returns_true_when_marker_set(self):
        exc = ScafldError("anything", code=EC.COMMAND_FAILED)
        exc._review_cancelled = True
        self.assertTrue(is_review_cancelled(exc))

    def test_returns_false_when_marker_absent(self):
        exc = ScafldError("external review runner failed via claude", code=EC.COMMAND_FAILED)
        self.assertFalse(is_review_cancelled(exc))

    def test_returns_false_when_marker_falsy(self):
        exc = ScafldError("anything", code=EC.COMMAND_FAILED)
        exc._review_cancelled = False
        self.assertFalse(is_review_cancelled(exc))

    def test_message_rewording_does_not_affect_helper(self):
        # Future rewording of the cancellation message must not regress the
        # cancel-UX dispatch path. The helper must rely on the marker only.
        exc1 = ScafldError("review interrupted", code=EC.COMMAND_FAILED)
        exc1._review_cancelled = True
        exc2 = ScafldError("operator pressed Ctrl-C", code=EC.COMMAND_FAILED)
        exc2._review_cancelled = True
        self.assertTrue(is_review_cancelled(exc1))
        self.assertTrue(is_review_cancelled(exc2))


def _canonical_review_packet_fixture(pass_ids):
    """Build a ReviewPacket that `normalize_review_packet` accepts."""
    return {
        "schema_version": "review_packet.v1",
        "review_summary": "Fixture summary",
        "verdict": "pass",
        "pass_results": {pass_id: "pass" for pass_id in pass_ids},
        "checked_surfaces": [
            {
                "pass_id": pass_id,
                "targets": [f"app.txt:1"],
                "summary": "fixture surface",
                "limitations": [],
            }
            for pass_id in pass_ids
        ],
        "findings": [],
    }


class ReviewPacketSchemaContractTest(unittest.TestCase):
    """Pin the static schema against the runtime contract."""

    def _load_schema(self):
        from pathlib import Path
        import json
        schema_path = Path(__file__).resolve().parents[1] / ".ai" / "schemas" / "review_packet.json"
        return json.loads(schema_path.read_text(encoding="utf-8"))

    def test_schema_file_loads_as_valid_json(self):
        schema = self._load_schema()
        self.assertEqual(schema["title"], "scafld ReviewPacket")
        self.assertEqual(
            schema["properties"]["schema_version"]["const"],
            "review_packet.v1",
        )

    def test_canonical_fixture_passes_normalize_and_schema_shape(self):
        from scafld.review_packet import normalize_review_packet
        pass_ids = ["regression_hunt", "convention_check", "dark_patterns"]
        topology = [
            {"id": pass_id, "kind": "adversarial", "title": pass_id, "description": "fixture", "order": idx * 10, "prompt": "fixture"}
            for idx, pass_id in enumerate(pass_ids, start=1)
        ]
        fixture = _canonical_review_packet_fixture(pass_ids)
        # Runtime validator accepts it
        normalize_review_packet(fixture, topology, root=None)
        # Schema-shape sanity: required top-level fields are present and verdict is in enum
        schema = self._load_schema()
        for required in schema["required"]:
            self.assertIn(required, fixture, f"fixture missing required schema field: {required}")
        self.assertIn(fixture["verdict"], schema["properties"]["verdict"]["enum"])

    def test_jsonschema_validation_when_available(self):
        try:
            import jsonschema  # type: ignore
        except ImportError:
            self.skipTest("jsonschema library not installed")
        pass_ids = ["regression_hunt", "convention_check"]
        fixture = _canonical_review_packet_fixture(pass_ids)
        jsonschema.validate(instance=fixture, schema=self._load_schema())

    def test_schema_rejects_unknown_top_level_field(self):
        try:
            import jsonschema  # type: ignore
        except ImportError:
            self.skipTest("jsonschema library not installed")
        pass_ids = ["regression_hunt"]
        fixture = _canonical_review_packet_fixture(pass_ids)
        fixture["bogus_field"] = "should be rejected"
        with self.assertRaises(jsonschema.ValidationError):
            jsonschema.validate(instance=fixture, schema=self._load_schema())

    def test_schema_rejects_invalid_verdict(self):
        try:
            import jsonschema  # type: ignore
        except ImportError:
            self.skipTest("jsonschema library not installed")
        pass_ids = ["regression_hunt"]
        fixture = _canonical_review_packet_fixture(pass_ids)
        fixture["verdict"] = "maybe"
        with self.assertRaises(jsonschema.ValidationError):
            jsonschema.validate(instance=fixture, schema=self._load_schema())


class ProviderArgSchemaTest(unittest.TestCase):
    """Verify the schema flag plumbing for both providers."""

    def test_codex_args_includes_output_schema_when_path_given(self):
        from scafld.review_runner import _codex_args
        args = _codex_args("/tmp/repo", "/tmp/out.txt", "gpt-5.5", schema_path="/tmp/schema.json")
        self.assertIn("--output-schema", args)
        self.assertEqual(args[args.index("--output-schema") + 1], "/tmp/schema.json")

    def test_codex_args_omits_output_schema_when_none(self):
        from scafld.review_runner import _codex_args
        args = _codex_args("/tmp/repo", "/tmp/out.txt", "gpt-5.5")
        self.assertNotIn("--output-schema", args)

    def test_claude_args_includes_json_schema_when_inline_given(self):
        from scafld.review_runner import _claude_args
        sid = "11111111-1111-4111-8111-111111111111"
        schema_str = '{"type":"object"}'
        args = _claude_args("claude-opus-4-7", sid, schema_json=schema_str)
        self.assertIn("--json-schema", args)
        self.assertEqual(args[args.index("--json-schema") + 1], schema_str)

    def test_claude_args_omits_json_schema_when_none(self):
        from scafld.review_runner import _claude_args
        sid = "11111111-1111-4111-8111-111111111111"
        args = _claude_args("claude-opus-4-7", sid)
        self.assertNotIn("--json-schema", args)


class ComposeReviewPacketSchemaTest(unittest.TestCase):
    """Verify topology pass_id parameterization at compose time."""

    def _topology(self, pass_ids):
        return [
            {"id": pid, "kind": "adversarial", "title": pid, "description": "fixture", "order": idx * 10, "prompt": "fixture"}
            for idx, pid in enumerate(pass_ids, start=1)
        ]

    def test_pass_results_properties_match_topology_pass_ids(self):
        from scafld.review_runner import _compose_review_packet_schema
        from pathlib import Path
        repo_root = Path(__file__).resolve().parents[1]
        topology = self._topology(["regression_hunt", "convention_check"])
        schema = _compose_review_packet_schema(repo_root, topology)
        pass_props = schema["properties"]["pass_results"]
        self.assertEqual(set(pass_props["properties"].keys()), {"regression_hunt", "convention_check"})
        self.assertEqual(set(pass_props["required"]), {"regression_hunt", "convention_check"})
        self.assertFalse(pass_props["additionalProperties"])

    def test_pass_results_value_enum_is_pass_fail_pwi(self):
        from scafld.review_runner import _compose_review_packet_schema
        from pathlib import Path
        repo_root = Path(__file__).resolve().parents[1]
        topology = self._topology(["regression_hunt"])
        schema = _compose_review_packet_schema(repo_root, topology)
        rh = schema["properties"]["pass_results"]["properties"]["regression_hunt"]
        self.assertEqual(set(rh["enum"]), {"pass", "fail", "pass_with_issues"})

    def test_findings_pass_id_is_narrowed_to_topology(self):
        from scafld.review_runner import _compose_review_packet_schema
        from pathlib import Path
        repo_root = Path(__file__).resolve().parents[1]
        topology = self._topology(["regression_hunt", "convention_check"])
        schema = _compose_review_packet_schema(repo_root, topology)
        f_pass_id = schema["properties"]["findings"]["items"]["properties"]["pass_id"]
        self.assertEqual(set(f_pass_id["enum"]), {"regression_hunt", "convention_check"})

    def test_checked_surfaces_pass_id_is_narrowed_to_topology(self):
        from scafld.review_runner import _compose_review_packet_schema
        from pathlib import Path
        repo_root = Path(__file__).resolve().parents[1]
        topology = self._topology(["regression_hunt", "dark_patterns"])
        schema = _compose_review_packet_schema(repo_root, topology)
        cs_pass_id = schema["properties"]["checked_surfaces"]["items"]["properties"]["pass_id"]
        self.assertEqual(set(cs_pass_id["enum"]), {"regression_hunt", "dark_patterns"})

    def test_overridden_schema_missing_keys_still_gets_narrowed(self):
        # If an operator overrides .ai/schemas/review_packet.json with a schema
        # that's missing findings/checked_surfaces structure, the composer must
        # still install the pass_id narrowing (no silent no-op).
        from scafld.review_runner import _compose_review_packet_schema
        from unittest.mock import patch
        sparse = {"$schema": "http://json-schema.org/draft-07/schema#", "type": "object", "properties": {}}
        with patch("scafld.review_runner.json.load", return_value=sparse), \
             patch("builtins.open", create=True), \
             patch("scafld.review_runner.resolve_review_packet_schema_path", return_value="/nonexistent"):
            topology = self._topology(["regression_hunt"])
            schema = _compose_review_packet_schema("/tmp/repo", topology)
        self.assertEqual(
            schema["properties"]["findings"]["items"]["properties"]["pass_id"]["enum"],
            ["regression_hunt"],
        )
        self.assertEqual(
            schema["properties"]["checked_surfaces"]["items"]["properties"]["pass_id"]["enum"],
            ["regression_hunt"],
        )


class PromptDoesNotInstructJsonOnlyTest(unittest.TestCase):
    """The prompt must not contain the redundant 'JSON only / no markdown'
    negation; the provider CLI enforces structure now."""

    def _build(self):
        from scafld.review_runner import build_external_review_prompt
        topology = [
            {"id": "regression_hunt", "kind": "adversarial", "title": "Regression Hunt", "description": "x", "order": 30, "prompt": "x"},
        ]
        return build_external_review_prompt("review prompt body", topology)

    def test_prompt_no_longer_says_do_not_return_markdown(self):
        prompt = self._build()
        self.assertNotIn("Do not return markdown", prompt)

    def test_prompt_no_longer_says_return_only_the_final(self):
        prompt = self._build()
        self.assertNotIn("Return only the final ReviewPacket JSON object now", prompt)

    def test_prompt_mentions_provider_cli_enforcement(self):
        prompt = self._build()
        self.assertIn("provider CLI", prompt)


class ExtractClaudeStdoutStructuredOutputTest(unittest.TestCase):
    """When claude is invoked with --json-schema, it returns the structured
    response in `structured_output`, not in `result`. The extractor must
    prefer that field."""

    def test_structured_output_is_preferred_over_result(self):
        wrapper = {
            "type": "result",
            "subtype": "success",
            "is_error": False,
            "result": "ReviewPacket emitted (text summary)",
            "structured_output": {
                "schema_version": "review_packet.v1",
                "verdict": "pass",
                "pass_results": {"regression_hunt": "pass"},
            },
            "session_id": "11111111-1111-4111-8111-111111111111",
        }
        raw, _model, _source, session = _extract_claude_stdout(json.dumps(wrapper))
        parsed = json.loads(raw)
        self.assertEqual(parsed["schema_version"], "review_packet.v1")
        self.assertEqual(parsed["verdict"], "pass")
        self.assertEqual(session, "11111111-1111-4111-8111-111111111111")

    def test_falls_back_to_result_when_no_structured_output(self):
        wrapper = {
            "type": "result",
            "result": '{"schema_version": "review_packet.v1", "verdict": "pass"}',
        }
        raw, _model, _source, _session = _extract_claude_stdout(json.dumps(wrapper))
        parsed = json.loads(raw)
        self.assertEqual(parsed["verdict"], "pass")

    def test_structured_output_must_be_dict_to_use(self):
        # If structured_output is somehow not a dict (regression in claude
        # output shape), fall through to result.
        wrapper = {
            "result": '{"verdict": "pass", "schema_version": "review_packet.v1"}',
            "structured_output": "not a dict",
        }
        raw, _m, _s, _sid = _extract_claude_stdout(json.dumps(wrapper))
        self.assertIn("verdict", raw)


class RedactedArgvTest(unittest.TestCase):
    """Verify long JSON values are redacted from the recorded command."""

    def test_json_schema_value_is_redacted(self):
        from scafld.review_runner import _redacted_argv
        argv = [
            "/opt/homebrew/bin/claude",
            "-p",
            "--json-schema",
            '{"type":"object","properties":{"verdict":{"enum":["pass","fail"]}}}',
            "--model",
            "claude-opus-4-7",
        ]
        rendered = _redacted_argv("/tmp/repo", argv)
        self.assertIn("<json>", rendered)
        self.assertNotIn('"type":"object"', rendered)
        self.assertIn("--json-schema", rendered)
        self.assertIn("--model", rendered)
        self.assertIn("claude-opus-4-7", rendered)

    def test_mcp_config_value_is_redacted(self):
        from scafld.review_runner import _redacted_argv
        argv = [
            "/opt/homebrew/bin/claude",
            "--mcp-config",
            '{"mcpServers":{}}',
            "-p",
        ]
        rendered = _redacted_argv("/tmp/repo", argv)
        self.assertIn("<json>", rendered)
        self.assertNotIn('"mcpServers"', rendered)


class WatchdogElapsedTest(unittest.TestCase):
    """One focused test for the suspend semantics. _watchdog_elapsed picks
    the larger of (wall_elapsed, monotonic_elapsed) so a laptop sleep
    counts toward the deadline."""

    def test_wall_jump_during_suspend_dominates_frozen_monotonic(self):
        from scafld import review_runner
        with unittest.mock.patch.object(review_runner.time, "time", return_value=8200.0), \
             unittest.mock.patch.object(review_runner.time, "monotonic", return_value=500.0):
            # start_wall=1000, start_mono=500. After "suspend": wall=8200, mono=500.
            # wall_elapsed = 7200, monotonic_elapsed = 0. max = 7200.
            self.assertEqual(review_runner._watchdog_elapsed(1000.0, 500.0), 7200.0)

    def test_monotonic_active_advance_with_no_wall_jump(self):
        from scafld import review_runner
        with unittest.mock.patch.object(review_runner.time, "time", return_value=1005.0), \
             unittest.mock.patch.object(review_runner.time, "monotonic", return_value=505.0):
            self.assertEqual(review_runner._watchdog_elapsed(1000.0, 500.0), 5.0)


class WatchdogSuspendIntegrationTest(unittest.TestCase):
    """One focused test for the watchdog's suspend behavior end-to-end:
    wall-clock jump during a simulated laptop suspend dominates the
    frozen monotonic reading, so absolute_max fires immediately and the
    watchdog terminates the proc.
    """

    def test_watchdog_fires_immediately_when_wall_clock_jumped_past_absolute_max(self):
        from scafld import review_runner

        proc = _make_proc([None, 0])  # alive at first poll, exits during grace
        done = threading.Event()
        # Start monotonic 500, time 1000. After suspend: time jumps to 8200
        # (7200s elapsed wall), monotonic stays around 500. Activity clock
        # also "frozen" at start so neither idle nor absolute_max would fire
        # if we used monotonic alone — but wall says 7200s > 600 absolute_max.
        # Sequence is: start_wall, start_mono, then per-iteration calls. The
        # exact number of time.time / time.monotonic calls is an
        # implementation detail; we use side_effect with a long head-pad and
        # let the rest stick at the suspended values.
        suspended_time = 8200.0
        suspended_mono = 500.5
        time_seq = [1000.0, suspended_time]  # start, first iter check
        mono_seq = [500.0, suspended_mono]
        time_calls = {"i": 0}
        mono_calls = {"i": 0}

        def _time():
            i = time_calls["i"]
            time_calls["i"] = i + 1
            return time_seq[i] if i < len(time_seq) else suspended_time

        def _mono():
            i = mono_calls["i"]
            mono_calls["i"] = i + 1
            return mono_seq[i] if i < len(mono_seq) else suspended_mono

        activity = {"last_wall": 1000.0, "last_mono": 500.0}
        kill_state = _empty_kill_state()
        with unittest.mock.patch.object(review_runner.time, "time", side_effect=_time), \
             unittest.mock.patch.object(review_runner.time, "monotonic", side_effect=_mono), \
             unittest.mock.patch("scafld.review_runner._terminate_provider_process_group") as term, \
             unittest.mock.patch("scafld.review_runner._kill_provider_process_group") as kill:
            review_runner._provider_watchdog(
                proc,
                idle_timeout_seconds=10000,
                absolute_max_seconds=600,
                activity=activity,
                done=done,
                kill_state=kill_state,
                sleep=lambda _s: None,
            )
        term.assert_called_once_with(proc)
        kill.assert_not_called()
        self.assertEqual(kill_state["reason"], "absolute_max")


class SigintEscalationTest(unittest.TestCase):
    """The SIGINT cancel path must escalate to SIGKILL after a short
    grace so subprocess descendants (e.g. `bash -lc "cat; sleep 30"`)
    that survive SIGTERM don't keep proc.communicate() blocked.

    We don't fire a real signal here; we exercise the escalation
    helpers used inside the SIGINT handler. Guards against the macOS
    cancel-signal flake where bash subprocesses survive SIGTERM long
    enough to wedge proc.communicate()."""

    def test_escalation_grace_is_small_positive(self):
        from scafld.review_runner import _SIGINT_KILL_GRACE_SECONDS
        self.assertGreater(_SIGINT_KILL_GRACE_SECONDS, 0)
        self.assertLess(_SIGINT_KILL_GRACE_SECONDS, 1.0)

    def test_kill_helper_is_idempotent_when_already_exited(self):
        # Mirrors the escalation timer's poll check: if SIGTERM was
        # honored and the subprocess already exited, the SIGKILL path
        # short-circuits — important so we never SIGKILL a reused pid.
        from scafld.review_runner import _kill_provider_process_group

        class _Proc:
            def __init__(self):
                self.killed = False
            def poll(self):
                return 0
            def kill(self):
                self.killed = True
            @property
            def pid(self):
                return 99999

        proc = _Proc()
        _kill_provider_process_group(proc)
        self.assertFalse(proc.killed)


class TimeoutResolutionTest(unittest.TestCase):
    """Config knobs: new fields take precedence; legacy `timeout_seconds`
    aliases to `absolute_max_seconds` with a deprecation note; defaults
    apply when nothing is configured."""

    def setUp(self):
        # Reset the once-per-process deprecation latch so each test sees a
        # fresh state. Smokes run scafld in fresh processes so this latch
        # resetting in tests is just hygiene.
        from scafld import review_runner
        review_runner._LEGACY_TIMEOUT_DEPRECATION_NOTED = False

    def test_defaults_when_nothing_configured(self):
        from scafld.review_runner import (
            DEFAULT_IDLE_TIMEOUT_SECONDS,
            DEFAULT_ABSOLUTE_MAX_SECONDS,
        )
        idle, abs_max = _resolve_review_timeouts({})
        self.assertEqual(idle, DEFAULT_IDLE_TIMEOUT_SECONDS)
        self.assertEqual(abs_max, DEFAULT_ABSOLUTE_MAX_SECONDS)

    def test_legacy_timeout_seconds_aliases_absolute_max(self):
        from scafld.review_runner import DEFAULT_IDLE_TIMEOUT_SECONDS
        captured = []

        def _fake_print(*args, **kwargs):
            captured.append(" ".join(str(a) for a in args))

        with unittest.mock.patch("builtins.print", side_effect=_fake_print):
            idle, abs_max = _resolve_review_timeouts({"timeout_seconds": 900})

        self.assertEqual(idle, DEFAULT_IDLE_TIMEOUT_SECONDS)
        self.assertEqual(abs_max, 900)
        # Deprecation note emitted exactly once.
        deprecation_lines = [c for c in captured if "timeout_seconds is deprecated" in c]
        self.assertEqual(len(deprecation_lines), 1)

    def test_new_keys_take_precedence_over_legacy(self):
        captured = []

        def _fake_print(*args, **kwargs):
            captured.append(" ".join(str(a) for a in args))

        with unittest.mock.patch("builtins.print", side_effect=_fake_print):
            idle, abs_max = _resolve_review_timeouts({
                "idle_timeout_seconds": 60,
                "absolute_max_seconds": 300,
                "timeout_seconds": 900,  # ignored when new keys present
            })

        self.assertEqual(idle, 60)
        self.assertEqual(abs_max, 300)
        # No deprecation note: legacy was silently ignored because the new
        # keys carry the operator's intent.
        deprecation_lines = [c for c in captured if "timeout_seconds is deprecated" in c]
        self.assertEqual(len(deprecation_lines), 0)

    def test_idle_cannot_exceed_absolute_max(self):
        with self.assertRaises(ValueError) as ctx:
            _resolve_review_timeouts({
                "idle_timeout_seconds": 600,
                "absolute_max_seconds": 60,
            })
        self.assertIn("idle_timeout_seconds", str(ctx.exception))
        self.assertIn("absolute_max_seconds", str(ctx.exception))

    def test_deprecation_note_only_emitted_once(self):
        # Three separate aliasings produce a single deprecation note.
        # Test the latch.
        captured = []

        def _fake_print(*args, **kwargs):
            captured.append(" ".join(str(a) for a in args))

        with unittest.mock.patch("builtins.print", side_effect=_fake_print):
            _resolve_review_timeouts({"timeout_seconds": 60})
            _resolve_review_timeouts({"timeout_seconds": 900})
            _resolve_review_timeouts({"timeout_seconds": 1200})

        deprecation_lines = [c for c in captured if "timeout_seconds is deprecated" in c]
        self.assertEqual(len(deprecation_lines), 1)

    def test_legacy_timeout_below_default_idle_does_not_reject(self):
        # Existing configs with `timeout_seconds: 60` (below the 120s
        # default idle) must still load — they predate the new keys and
        # the operator never set the idle threshold. The default idle
        # clamps to absolute_max in that case so the cross-check passes.
        idle, abs_max = _resolve_review_timeouts({"timeout_seconds": 60})
        self.assertEqual(abs_max, 60)
        self.assertEqual(idle, 60)
        self.assertLessEqual(idle, abs_max)

    def test_legacy_small_timeout_with_default_idle_clamps(self):
        idle, abs_max = _resolve_review_timeouts({"timeout_seconds": 30})
        self.assertEqual(abs_max, 30)
        self.assertEqual(idle, 30)



class StreamingProviderTransportTest(unittest.TestCase):
    """End-to-end transport behaviour with a real subprocess. Each test
    spawns python3 with a tiny inline program so the streaming/idle/abs_max
    semantics are exercised through the real reader-thread pipeline."""

    REPO_ROOT = Path(__file__).resolve().parent.parent

    def _python_argv(self, code):
        return [sys.executable, "-c", code]

    def test_streams_stdout_through_pump(self):
        # Print 5 ticks at 50ms intervals with explicit flush so the reader
        # thread sees bytes incrementally. With idle_timeout=10s and
        # absolute_max=10s, neither threshold fires and the run completes.
        code = (
            "import sys, time\n"
            "for i in range(5):\n"
            "    print(f'tick {i}', flush=True)\n"
            "    time.sleep(0.05)\n"
            "print('done', flush=True)\n"
        )
        result = _run_provider_subprocess(
            self._python_argv(code),
            prompt="",
            root=self.REPO_ROOT,
            provider="codex",
            idle_timeout_seconds=10,
            absolute_max_seconds=10,
        )
        self.assertEqual(result.returncode, 0)
        self.assertIn("tick 0", result.stdout)
        self.assertIn("tick 4", result.stdout)
        self.assertIn("done", result.stdout)
        self.assertIsNone(result.kill_reason)
        self.assertFalse(result.timed_out)

    def test_kills_on_idle_timeout(self):
        # Print one line, then sleep silently. idle_timeout=0.5s should fire
        # within ~1s; absolute_max=30s is the safety net (must not fire).
        code = (
            "import sys, time\n"
            "print('hello', flush=True)\n"
            "time.sleep(30)\n"
        )
        result = _run_provider_subprocess(
            self._python_argv(code),
            prompt="",
            root=self.REPO_ROOT,
            provider="codex",
            idle_timeout_seconds=0.5,
            absolute_max_seconds=30,
        )
        self.assertEqual(result.kill_reason, "idle_timeout")
        self.assertTrue(result.timed_out)
        self.assertIn("hello", result.stdout)
        # The recorded idle age should be at or above the threshold; the
        # exact value depends on poll cadence so allow a small slack.
        self.assertGreaterEqual(result.time_since_last_byte, 0.5)

    def test_kills_on_absolute_max_even_with_continuous_output(self):
        # Stream a byte every 50ms forever. idle_timeout=10s never fires
        # (each tick refreshes activity); absolute_max=1s does fire.
        code = (
            "import sys, time\n"
            "i = 0\n"
            "while True:\n"
            "    print(f't{i}', flush=True)\n"
            "    time.sleep(0.05)\n"
            "    i += 1\n"
        )
        result = _run_provider_subprocess(
            self._python_argv(code),
            prompt="",
            root=self.REPO_ROOT,
            provider="codex",
            idle_timeout_seconds=10,
            absolute_max_seconds=1.0,
        )
        self.assertEqual(result.kill_reason, "absolute_max")
        self.assertTrue(result.timed_out)
        self.assertIn("t0", result.stdout)

    def test_long_productive_run_not_killed(self):
        # 2 seconds of streaming with frequent ticks. idle_timeout=0.5s
        # would have fired the old wall-clock-only kill; the new transport
        # sees activity each tick and lets the process complete.
        code = (
            "import sys, time\n"
            "for i in range(20):\n"
            "    print(f't{i}', flush=True)\n"
            "    time.sleep(0.1)\n"
        )
        result = _run_provider_subprocess(
            self._python_argv(code),
            prompt="",
            root=self.REPO_ROOT,
            provider="codex",
            idle_timeout_seconds=0.5,
            absolute_max_seconds=10.0,
        )
        self.assertEqual(result.returncode, 0)
        self.assertIsNone(result.kill_reason)
        self.assertIn("t0", result.stdout)
        self.assertIn("t19", result.stdout)
        self.assertFalse(result.timed_out)

    def test_records_idle_and_abs_max_in_result(self):
        # Even a healthy run records the threshold values so the diagnostic
        # can reproduce the operator's effective configuration.
        code = "print('ok', flush=True)"
        result = _run_provider_subprocess(
            self._python_argv(code),
            prompt="",
            root=self.REPO_ROOT,
            provider="codex",
            idle_timeout_seconds=120,
            absolute_max_seconds=1800,
        )
        self.assertEqual(result.idle_timeout_seconds, 120)
        self.assertEqual(result.absolute_max_seconds, 1800)

    def test_codex_style_split_io_works(self):
        # Codex's `exec` mode streams human-readable progress to stdout AND
        # writes the structured payload to the file passed via
        # `--output-last-message`. The transport handles stdin/stdout/stderr;
        # the outer call site reads the file. This test simulates that
        # split-IO pattern: stdout streams progress, a separate file gets
        # the "structured" payload. Verifies the new transport doesn't
        # interfere with codex's file-based payload delivery.
        import tempfile
        with tempfile.NamedTemporaryFile(mode="w", suffix=".txt", delete=False) as tmp:
            payload_path = tmp.name
        code = (
            "import sys, time\n"
            "for i in range(3):\n"
            "    print(f'progress {i}', flush=True)\n"
            "    time.sleep(0.05)\n"
            f"open({payload_path!r}, 'w').write('{{\"verdict\": \"pass\"}}')\n"
            "print('done', flush=True)\n"
        )
        result = _run_provider_subprocess(
            self._python_argv(code),
            prompt="",
            root=self.REPO_ROOT,
            provider="codex",
            idle_timeout_seconds=10,
            absolute_max_seconds=10,
        )
        self.assertEqual(result.returncode, 0)
        # stdout streamed naturally
        self.assertIn("progress 0", result.stdout)
        self.assertIn("done", result.stdout)
        # external file got the structured payload
        with open(payload_path) as f:
            payload = f.read()
        self.assertEqual(json.loads(payload), {"verdict": "pass"})
        Path(payload_path).unlink()


class WatchdogKillDiagnosticTest(unittest.TestCase):
    """When the watchdog kills the provider, the diagnostic file must
    record kill_reason, time_since_last_byte, the threshold values in
    effect, the parsed event summary (claude NDJSON), and the tail of
    stdout — so the operator has an actionable trail instead of empty
    fields."""

    def _root_with_diag_dir(self, tmp_dir):
        from scafld.runtime_contracts import diagnostics_dir
        root = Path(tmp_dir)
        # `diagnostics_dir` builds .ai/runs/<task>/diagnostics — make sure
        # the parents exist so `_write_external_diagnostic` can write.
        diag = diagnostics_dir(root, "sometask")
        diag.parent.mkdir(parents=True, exist_ok=True)
        return root

    def _write_diag(self, root, **overrides):
        from scafld.review_runner import _write_external_diagnostic
        defaults = dict(
            provider="claude",
            provider_requested="auto",
            argv=["claude", "-p"],
            started_at="2026-04-28T00:00:00Z",
            completed_at="2026-04-28T00:10:00Z",
            exit_code=143,
            timed_out=True,
            timeout_seconds=1800,
            prompt_sha256="abc",
            prompt_preview="prompt",
            stdout="hello world\n",
            stderr="",
            raw_output="",
            error="provider timed out",
        )
        defaults.update(overrides)
        path = _write_external_diagnostic(root, "sometask", **defaults)
        return (root / path).read_text(encoding="utf-8")

    def test_idle_timeout_kill_records_section(self):
        import tempfile
        with tempfile.TemporaryDirectory() as tmp:
            root = self._root_with_diag_dir(tmp)
            body = self._write_diag(
                root,
                kill_reason="idle_timeout",
                time_since_last_byte=125.4,
                idle_timeout_seconds=120,
                absolute_max_seconds=1800,
                parsed_event_summary={"system.init": 1, "assistant": 14},
            )
        self.assertIn("## Watchdog Kill", body)
        self.assertIn("kill_reason: idle_timeout", body)
        self.assertIn("time_since_last_byte: 125.40", body)
        self.assertIn("idle_timeout_seconds: 120", body)
        self.assertIn("absolute_max_seconds: 1800", body)
        self.assertIn("events: 14× assistant, 1× system.init", body)
        self.assertIn("### Stdout Tail", body)

    def test_absolute_max_kill_records_section(self):
        import tempfile
        with tempfile.TemporaryDirectory() as tmp:
            root = self._root_with_diag_dir(tmp)
            body = self._write_diag(
                root,
                kill_reason="absolute_max",
                time_since_last_byte=2.1,
                idle_timeout_seconds=120,
                absolute_max_seconds=300,
                parsed_event_summary={"system.init": 1, "assistant": 60, "result.success": 0},
            )
        self.assertIn("## Watchdog Kill", body)
        self.assertIn("kill_reason: absolute_max", body)
        self.assertIn("absolute_max_seconds: 300", body)

    def test_no_watchdog_section_when_not_timed_out(self):
        # Regression guard — non-timeout failures (workspace mutation,
        # spawn failure, model rejection) must NOT grow a misleading
        # "## Watchdog Kill" header.
        import tempfile
        with tempfile.TemporaryDirectory() as tmp:
            root = self._root_with_diag_dir(tmp)
            body = self._write_diag(
                root,
                timed_out=False,
                exit_code=1,
                error="provider rejected model",
                kill_reason=None,
            )
        self.assertNotIn("## Watchdog Kill", body)

    def test_no_watchdog_section_when_timed_out_but_no_reason(self):
        # Defensive: timed_out=True with no kill_reason (legacy path or
        # unknown) → omit the new section to avoid showing empty fields.
        import tempfile
        with tempfile.TemporaryDirectory() as tmp:
            root = self._root_with_diag_dir(tmp)
            body = self._write_diag(
                root,
                timed_out=True,
                kill_reason=None,
            )
        self.assertNotIn("## Watchdog Kill", body)

    def test_event_summary_empty_renders_none_marker(self):
        import tempfile
        with tempfile.TemporaryDirectory() as tmp:
            root = self._root_with_diag_dir(tmp)
            body = self._write_diag(
                root,
                kill_reason="idle_timeout",
                time_since_last_byte=130.0,
                idle_timeout_seconds=120,
                absolute_max_seconds=1800,
                parsed_event_summary=None,
            )
        self.assertIn("events: (none parsed)", body)

    def test_stdout_tail_truncates_long_output(self):
        # 32KB of output → tail captures last 16KB plus a "head trimmed"
        # marker so the operator can tell the diagnostic was sampled.
        import tempfile
        long_stdout = ("A" * 17000) + ("B" * 17000)
        with tempfile.TemporaryDirectory() as tmp:
            root = self._root_with_diag_dir(tmp)
            body = self._write_diag(
                root,
                stdout=long_stdout,
                kill_reason="idle_timeout",
                time_since_last_byte=130.0,
                idle_timeout_seconds=120,
                absolute_max_seconds=1800,
                parsed_event_summary={"assistant": 1},
            )
        # The Watchdog Kill section's Stdout Tail should have the trim
        # marker and the trailing 'B' bytes, not the leading 'A' run.
        watchdog_section = body.split("## Watchdog Kill", 1)[1]
        self.assertIn("[head trimmed", watchdog_section)
        self.assertIn("B" * 100, watchdog_section)

    def test_threshold_lines_in_main_header_when_set(self):
        # Even on healthy runs, recording the active idle/abs_max values
        # in the main header lets the operator confirm the effective
        # configuration without parsing the Watchdog Kill section.
        import tempfile
        with tempfile.TemporaryDirectory() as tmp:
            root = self._root_with_diag_dir(tmp)
            body = self._write_diag(
                root,
                timed_out=False,
                exit_code=0,
                error="",
                idle_timeout_seconds=120,
                absolute_max_seconds=1800,
            )
        self.assertIn("idle_timeout_seconds: 120", body)
        self.assertIn("absolute_max_seconds: 1800", body)


if __name__ == "__main__":
    unittest.main()
