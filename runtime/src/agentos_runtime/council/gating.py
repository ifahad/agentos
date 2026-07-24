"""Action gating for the council: reads run, writes become proposals.

An autonomous council running unattended across several third-party models is
exactly where a prompt injection in a retrieved document has the most reach. So
the council's action surface is: read freely, propose everything else.

Classification is FAIL-CLOSED. Only tools on the read-safe allowlist execute
unattended; anything else — including a tool added tomorrow that nobody
classified — is held as a proposal for a human.

NOTE: deepagents compiles its own graph with no interrupt_before pass-through,
so in-graph write gating applies to react-profile members. Deep-profile members
must be given read-only tools via the member's ``tools:`` list — enforce the
action surface by tool selection there. The shipped council.yaml is asserted to
follow this by a test.
"""

from __future__ import annotations

import logging
from collections.abc import Sequence

from agentos_runtime.hitl import PendingCall, deny_pending

logger = logging.getLogger(__name__)

# Tools the council may execute unattended. Each is read-only by construction:
#   query/list_tables/describe_table - SQL connector, read-only enforced and
#     wrapped in a READ ONLY transaction upstream
#   search_knowledge                 - pgvector retrieval over indexed documents
#   run_python                       - the Rust sandbox: egress-less, read-only
#     rootfs, all capabilities dropped, rlimited
# Adding a name here is a governance decision, not a formatting change.
READ_SAFE_TOOLS = frozenset(
    {
        "query",
        "list_tables",
        "describe_table",
        "search_knowledge",
        "run_python",
    }
)

PROPOSAL_PENDING = "pending"


def is_read_safe(tool_name: str) -> bool:
    """Whether a tool may execute without human approval."""
    return tool_name in READ_SAFE_TOOLS


def write_class_calls(calls: Sequence[PendingCall]) -> list[PendingCall]:
    """The subset of pending calls that must not execute unattended."""
    return [c for c in calls if not is_read_safe(c.tool)]


async def hold_writes_as_proposals(
    agent,
    config: dict,
    calls: Sequence[PendingCall],
    store,
    objective_id: str,
    member_id: str,
) -> list[str]:
    """Record write-class calls as proposals and deny them in-graph.

    Returns the created proposal ids. Denying via deny_pending makes the member
    observe that the tool did not run, so it reasons about the refusal instead of
    assuming success.
    """
    held = write_class_calls(calls)
    if not held:
        return []
    proposal_ids = []
    for pending in held:
        proposal_id = await store.insert_proposal(
            objective_id=objective_id,
            member_id=member_id,
            tool=pending.tool,
            arguments=dict(pending.input),
        )
        proposal_ids.append(proposal_id)
        logger.info(
            "council held write-class call %s from member %s as proposal %s",
            pending.tool, member_id, proposal_id,
        )
    await deny_pending(agent, config, list(held))
    return proposal_ids
