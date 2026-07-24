"""Judge synthesis: verdict parsing, dissent, and judge-outage handling."""

import json

import pytest

from agentos_runtime.council.fanout import MemberAnswer
from agentos_runtime.council.judge import (
    JUDGE_UNAVAILABLE,
    build_judge_prompt,
    parse_verdict,
    synthesize,
)


def answers():
    return [
        MemberAnswer("alpha", "ollama/a", "t1", "answered", "Riyadh owes 4,200 SAR", [], ""),
        MemberAnswer("beta", "ollama/b", "t2", "answered", "Riyadh owes 4,200 SAR", [], ""),
        MemberAnswer("gamma", "ollama/c", "t3", "answered", "Riyadh owes 9,900 SAR", [], ""),
    ]


class StubJudge:
    """Chat-model stand-in returning queued replies."""

    def __init__(self, *replies):
        self.replies = list(replies)
        self.calls = 0

    async def ainvoke(self, messages):
        self.calls += 1
        reply = self.replies.pop(0)
        if isinstance(reply, Exception):
            raise reply
        return StubMessage(reply)


class StubMessage:
    def __init__(self, content):
        self.content = content


def test_prompt_contains_every_answer_labelled_by_member():
    prompt = build_judge_prompt("who owes what?", answers())
    assert "who owes what?" in prompt
    for member in ("alpha", "beta", "gamma"):
        assert member in prompt
    assert "9,900" in prompt


def test_prompt_marks_member_answers_as_untrusted_data():
    """Member output is model-generated text; the judge must not obey it."""
    prompt = build_judge_prompt("q", answers())
    assert "untrusted" in prompt.lower()


def test_parse_verdict_reads_a_well_formed_reply():
    raw = json.dumps({
        "answer": "Riyadh owes 4,200 SAR",
        "agreement": 0.67,
        "dissent": [{"member": "gamma", "claim": "9,900 SAR", "basis": "different table"}],
        "cited_members": ["alpha", "beta"],
        "done": True,
    })
    verdict = parse_verdict(raw, answers())
    assert verdict.answer == "Riyadh owes 4,200 SAR"
    assert verdict.agreement == pytest.approx(0.67)
    assert verdict.dissent[0]["member"] == "gamma"
    assert verdict.cited_members == ["alpha", "beta"]
    assert verdict.done is True
    assert verdict.status == "ok"


def test_parse_verdict_tolerates_fenced_json():
    """Models wrap JSON in ```json fences; that must not fail the cycle."""
    raw = '```json\n{"answer":"x","agreement":1.0,"dissent":[],"cited_members":["alpha"]}\n```'
    verdict = parse_verdict(raw, answers())
    assert verdict.answer == "x"


def test_parse_verdict_rejects_unparseable():
    assert parse_verdict("I think the answer is probably 4200", answers()) is None
    assert parse_verdict("", answers()) is None


def test_parse_verdict_clamps_agreement_and_drops_unknown_members():
    raw = json.dumps({
        "answer": "x",
        "agreement": 4.2,
        "dissent": [{"member": "ghost", "claim": "c", "basis": "b"}],
        "cited_members": ["alpha", "nobody"],
    })
    verdict = parse_verdict(raw, answers())
    assert 0.0 <= verdict.agreement <= 1.0
    assert verdict.cited_members == ["alpha"]
    assert verdict.dissent == []


async def test_synthesize_returns_the_verdict():
    raw = json.dumps({"answer": "ok", "agreement": 1.0, "dissent": [], "cited_members": ["alpha"]})
    judge = StubJudge(raw)
    verdict = await synthesize(judge, "q", answers())
    assert verdict.answer == "ok"
    assert judge.calls == 1


async def test_synthesize_retries_once_on_unparseable_output():
    good = json.dumps({"answer": "ok", "agreement": 1.0, "dissent": [], "cited_members": ["alpha"]})
    judge = StubJudge("rambling prose", good)
    verdict = await synthesize(judge, "q", answers())
    assert verdict.answer == "ok"
    assert judge.calls == 2


async def test_synthesize_degrades_to_judge_unavailable():
    """A judge outage degrades ONE cycle; it never raises into the loop."""
    judge = StubJudge("nonsense", "still nonsense")
    verdict = await synthesize(judge, "q", answers())
    assert verdict.status == JUDGE_UNAVAILABLE
    assert verdict.agreement == 0.0
    assert judge.calls == 2


async def test_synthesize_survives_a_judge_exception():
    judge = StubJudge(RuntimeError("judge 503"), RuntimeError("judge 503 again"))
    verdict = await synthesize(judge, "q", answers())
    assert verdict.status == JUDGE_UNAVAILABLE


async def test_synthesize_ignores_failed_members():
    with_failure = answers() + [
        MemberAnswer("delta", "ollama/d", "t4", "failed", "", [], "provider 502")
    ]
    raw = json.dumps({"answer": "ok", "agreement": 1.0, "dissent": [], "cited_members": ["alpha"]})
    judge = StubJudge(raw)
    await synthesize(judge, "q", with_failure)
    prompt = build_judge_prompt("q", with_failure)
    assert "provider 502" not in prompt
    assert "delta" not in prompt
