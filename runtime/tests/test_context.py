"""Context engine tests: fake embeddings + in-memory vector store, no network."""

import math
from collections import Counter

import httpx
import pytest
from helpers import make_settings
from llama_index.core.base.embeddings.base import BaseEmbedding
from llama_index.core.vector_stores.types import VectorStoreQueryResult

from agentos_runtime.api import app
from agentos_runtime.context import (
    ContextEngine,
    GatewayEmbedding,
    build_context_engine,
    make_search_tool,
)

KEYWORDS = ["alpha", "beta", "gamma", "delta"]


class FakeEmbedding(BaseEmbedding):
    """Deterministic keyword-count embeddings; no network."""

    def _vector(self, text: str) -> list[float]:
        counts = [float(text.lower().count(k)) for k in KEYWORDS]
        norm = math.sqrt(sum(c * c for c in counts)) or 1.0
        return [c / norm for c in counts] + [1e-6]  # avoid zero vectors

    def _get_query_embedding(self, query: str) -> list[float]:
        return self._vector(query)

    async def _aget_query_embedding(self, query: str) -> list[float]:
        return self._vector(query)

    def _get_text_embedding(self, text: str) -> list[float]:
        return self._vector(text)

    async def _aget_text_embedding(self, text: str) -> list[float]:
        return self._vector(text)


class InMemoryVectorStore:
    """Minimal async vector store standing in for PGVectorStore."""

    def __init__(self):
        self.nodes = []

    async def async_add(self, nodes):
        self.nodes.extend(nodes)
        return [n.node_id for n in nodes]

    async def aquery(self, query):
        def score(node):
            return sum(a * b for a, b in zip(node.embedding, query.query_embedding, strict=True))

        ranked = sorted(self.nodes, key=score, reverse=True)[: query.similarity_top_k]
        return VectorStoreQueryResult(
            nodes=ranked, similarities=[score(n) for n in ranked], ids=[n.node_id for n in ranked]
        )

    async def counts(self):
        tally = Counter(n.metadata["name"] for n in self.nodes)
        return [{"name": name, "chunks": count} for name, count in sorted(tally.items())]


def make_engine():
    store = InMemoryVectorStore()
    return ContextEngine(FakeEmbedding(model_name="fake"), store, store.counts), store


async def test_add_document_single_chunk():
    engine, store = make_engine()
    assert await engine.add_document("note", "alpha is the first letter.") == 1
    assert store.nodes[0].metadata == {"name": "note"}


async def test_add_document_long_text_multiple_chunks():
    engine, _ = make_engine()
    text = "The alpha metric rose sharply during the audit period. " * 200
    chunks = await engine.add_document("long", text)
    assert chunks > 1
    assert [d["chunks"] for d in await engine.list_documents()] == [chunks]


async def test_list_documents_aggregates_by_name():
    engine, _ = make_engine()
    await engine.add_document("a", "alpha alpha")
    await engine.add_document("b", "beta beta")
    assert await engine.list_documents() == [
        {"name": "a", "chunks": 1},
        {"name": "b", "chunks": 1},
    ]


async def test_search_returns_top_chunks_with_sources():
    engine, _ = make_engine()
    await engine.add_document("alpha-doc", "alpha alpha alpha facts here")
    await engine.add_document("beta-doc", "beta beta beta facts here")
    results = await engine.search("tell me about alpha")
    assert results[0]["name"] == "alpha-doc"
    assert "alpha" in results[0]["text"]
    assert len(results) <= 5


async def test_search_knowledge_tool_formats_sources():
    engine, _ = make_engine()
    await engine.add_document("handbook", "gamma rules apply on gamma days")
    tool = make_search_tool(engine)
    assert tool.name == "search_knowledge"
    result = await tool.ainvoke({"query": "gamma"})
    assert "[source: handbook]" in result
    assert "gamma rules" in result


async def test_search_knowledge_tool_empty_store():
    engine, _ = make_engine()
    result = await make_search_tool(engine).ainvoke({"query": "anything"})
    assert result == "No matching documents found."


@pytest.fixture
async def client():
    engine, _ = make_engine()
    app.state.context_engine = engine
    transport = httpx.ASGITransport(app=app)
    async with httpx.AsyncClient(transport=transport, base_url="http://test") as c:
        yield c
    del app.state.context_engine


async def test_documents_endpoints(client):
    response = await client.post("/documents", json={"name": "doc1", "text": "delta notes"})
    assert response.status_code == 200
    assert response.json() == {"name": "doc1", "chunks": 1}
    listing = await client.get("/documents")
    assert listing.status_code == 200
    assert listing.json() == [{"name": "doc1", "chunks": 1}]


async def test_documents_503_when_engine_disabled():
    transport = httpx.ASGITransport(app=app)
    async with httpx.AsyncClient(transport=transport, base_url="http://test") as c:
        assert (await c.get("/documents")).status_code == 503
        assert (
            await c.post("/documents", json={"name": "x", "text": "y"})
        ).status_code == 503


def test_gateway_embedding_request_and_parse():
    embed = GatewayEmbedding(
        model_name="ollama/bge-m3",
        gateway_url="http://gateway:8080/",
        gateway_key="agos-key",
    )
    request = embed._request(["hello", "world"])
    assert request["url"] == "http://gateway:8080/v1/embeddings"
    assert request["json"] == {"model": "ollama/bge-m3", "input": ["hello", "world"]}
    assert request["headers"]["Authorization"] == "Bearer agos-key"
    response = httpx.Response(
        200,
        json={
            "data": [
                {"index": 1, "embedding": [0.2]},
                {"index": 0, "embedding": [0.1]},
            ]
        },
        request=httpx.Request("POST", request["url"]),
    )
    assert embed._parse(response) == [[0.1], [0.2]]


def test_build_context_engine_pg_construction():
    settings = make_settings(
        checkpoint_database_url="postgresql://agentos:pw@db:5432/agentos"
    )
    engine = build_context_engine(settings)
    from llama_index.vector_stores.postgres import PGVectorStore

    assert isinstance(engine.vector_store, PGVectorStore)
    assert engine.vector_store.table_name == "agentos_documents"
    assert isinstance(engine.embed_model, GatewayEmbedding)
    assert engine.embed_model.model_name == "ollama/bge-m3"


def test_context_engine_enabled_resolution():
    assert make_settings().context_engine_enabled is False
    assert make_settings(checkpoint_database_url="postgresql://x").context_engine_enabled
    assert make_settings(context_engine="on").context_engine_enabled is True
    assert (
        make_settings(
            context_engine="off", checkpoint_database_url="postgresql://x"
        ).context_engine_enabled
        is False
    )
