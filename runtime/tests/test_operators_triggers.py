"""Operator trigger validation and next-fire computation."""

import pytest

from agentos_runtime.operators.triggers import (
    MIN_INTERVAL_S,
    WEBHOOK_PREFIX,
    TriggerError,
    next_cron_fire,
    next_interval_fire,
    parse_trigger,
)


def test_interval_trigger_parses():
    t = parse_trigger({"type": "interval", "interval_s": 60})
    assert t.kind == "interval"
    assert t.interval_s == 60
    assert t.summary() == "every 60s"


def test_interval_below_floor_rejected():
    with pytest.raises(TriggerError, match="interval_s must be"):
        parse_trigger({"type": "interval", "interval_s": MIN_INTERVAL_S - 1})


def test_interval_non_integer_rejected():
    with pytest.raises(TriggerError, match="integer interval_s"):
        parse_trigger({"type": "interval", "interval_s": "soon"})


def test_cron_trigger_parses():
    t = parse_trigger({"type": "cron", "cron": "0 9 * * *"})
    assert t.kind == "cron"
    assert t.cron == "0 9 * * *"
    assert "cron" in t.summary()


def test_invalid_cron_rejected():
    with pytest.raises(TriggerError, match="invalid cron"):
        parse_trigger({"type": "cron", "cron": "not a cron"})


def test_missing_cron_rejected():
    with pytest.raises(TriggerError, match="needs a cron"):
        parse_trigger({"type": "cron"})


def test_webhook_mints_a_token():
    t = parse_trigger({"type": "webhook"})
    assert t.kind == "webhook"
    assert t.webhook_token.startswith(WEBHOOK_PREFIX)
    assert len(t.webhook_token) > len(WEBHOOK_PREFIX) + 10


def test_webhook_round_trips_an_existing_token():
    t = parse_trigger({"type": "webhook", "webhook_token": "whk-existing"})
    assert t.webhook_token == "whk-existing"


def test_webhook_token_never_taken_from_a_non_webhook():
    # A client cannot smuggle a token onto an interval trigger.
    t = parse_trigger({"type": "interval", "interval_s": 60, "webhook_token": "whk-evil"})
    assert t.webhook_token is None


def test_unknown_type_rejected():
    with pytest.raises(TriggerError, match="trigger type"):
        parse_trigger({"type": "telepathy"})


def test_trigger_must_be_an_object():
    with pytest.raises(TriggerError, match="must be an object"):
        parse_trigger("every hour")


def test_to_config_round_trips_through_parse():
    for raw in (
        {"type": "interval", "interval_s": 45},
        {"type": "cron", "cron": "*/5 * * * *"},
    ):
        t = parse_trigger(raw)
        again = parse_trigger({"type": t.kind, **t.to_config()})
        assert again == t


def test_next_interval_fire():
    # First fire (never fired) is due now; afterwards it's last + interval.
    assert next_interval_fire(60, last_fired=0.0, now=1000.0) == 1000.0
    assert next_interval_fire(60, last_fired=1000.0, now=1030.0) == 1060.0


def test_next_cron_fire_is_strictly_after():
    # 09:00 daily; from just before, the next fire is that 09:00.
    base = 1_700_000_000.0
    nxt = next_cron_fire("0 9 * * *", base)
    assert nxt > base
    # And iterating again advances.
    assert next_cron_fire("0 9 * * *", nxt) > nxt
