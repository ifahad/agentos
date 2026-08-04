"""Global test wiring.

The runtime API now enforces ``Authorization: Bearer <token>`` on every route
except ``GET /healthz`` and ``/operators/webhooks/{token}`` (finding C1), where
the ``whk-`` path segment is itself the credential. Set the expected token in
the environment once for the whole test session so the app-wide auth dependency
has a value to compare against; individual test clients send it via
``helpers.AUTH_HEADERS``.
"""

import os

from helpers import RUNTIME_AUTH_TOKEN

os.environ["AGENTOS_RUNTIME_AUTH_TOKEN"] = RUNTIME_AUTH_TOKEN
