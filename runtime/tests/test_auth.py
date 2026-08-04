"""Runtime authentication tests (finding C1).

Every route except ``GET /healthz`` and ``/operators/webhooks/{token}`` requires
``Authorization: Bearer <token>``, compared constant-time against
AGENTOS_RUNTIME_AUTH_TOKEN. The app refuses to start when the token is unset.
"""

import httpx
import pytest
from helpers import AUTH_HEADERS, make_settings

from agentos_runtime.api import app


class _FakeAgent:
    async def ainvoke(self, state, config=None):
        return {"messages": []}


@pytest.fixture
async def raw_client():
    """Client with NO default auth header, so each test controls the header."""
    app.state.agent = _FakeAgent()
    transport = httpx.ASGITransport(app=app)
    async with httpx.AsyncClient(transport=transport, base_url="http://test") as c:
        yield c
    del app.state.agent


async def test_protected_route_401_without_token(raw_client):
    response = await raw_client.post("/runs", json={"input": "hi"})
    assert response.status_code == 401
    assert response.json() == {"detail": "invalid runtime token"}


async def test_protected_route_401_with_wrong_token(raw_client):
    response = await raw_client.post(
        "/runs", json={"input": "hi"}, headers={"Authorization": "Bearer wrong-token"}
    )
    assert response.status_code == 401
    assert response.json() == {"detail": "invalid runtime token"}


async def test_protected_route_401_with_malformed_scheme(raw_client):
    # A bare token without the "Bearer " scheme is rejected.
    response = await raw_client.post(
        "/runs", json={"input": "hi"}, headers={"Authorization": "test-runtime-token"}
    )
    assert response.status_code == 401


async def test_protected_route_200_with_right_token(raw_client):
    response = await raw_client.post(
        "/runs", json={"input": "hi"}, headers=AUTH_HEADERS
    )
    assert response.status_code == 200


async def test_healthz_open_without_token(raw_client):
    response = await raw_client.get("/healthz")
    assert response.status_code == 200
    assert response.json() == {"status": "ok"}


def test_settings_refuses_start_without_token():
    with pytest.raises(RuntimeError, match="AGENTOS_RUNTIME_AUTH_TOKEN must be set"):
        make_settings(runtime_auth_token="").require_runtime_auth_token()


def test_settings_refuses_start_with_whitespace_token():
    with pytest.raises(RuntimeError, match="AGENTOS_RUNTIME_AUTH_TOKEN must be set"):
        make_settings(runtime_auth_token="   ").require_runtime_auth_token()


def test_settings_accepts_configured_token():
    assert make_settings(runtime_auth_token="s3cret").require_runtime_auth_token() == "s3cret"
