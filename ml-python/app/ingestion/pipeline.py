from qdrant_client import QdrantClient
from qdrant_client.models import Distance, VectorParams, PointStruct
from fastembed import TextEmbedding
from app.core.config import settings
from app.ingestion.chunker import TextChunker
from typing import List, Dict, Any

class IngestionPipeline:
    def __init__(self):
        self.client = QdrantClient(host=settings.QDRANT_HOST, port=settings.QDRANT_PORT)
        self.embed_model = TextEmbedding(model_name=settings.EMBEDDING_MODEL)
        self.chunker = TextChunker()
        self._ensure_collection()

    def _ensure_collection(self):
        collections = [c.name for c in self.client.get_collections().collections]
        if settings.COLLECTION_NAME not in collections:
            self.client.create_collection(
                collection_name=settings.COLLECTION_NAME,
                vectors_config=VectorParams(size=settings.VECTOR_SIZE, distance=Distance.COSINE),
            )

    def process_and_store(self, doc_id: str, content: str, metadata: Dict[str, Any] = None) -> int:
        chunks = self.chunker.split_text(content, doc_id, metadata)
        if not chunks:
            return 0

        texts = [c["content"] for c in chunks]
        embeddings = list(self.embed_model.embed(texts))

        points = []
        for i, chunk in enumerate(chunks):
            points.append(
                PointStruct(
                    id=chunk["chunk_id"],
                    vector=embeddings[i].tolist(),
                    payload={
                        "doc_id": chunk["doc_id"],
                        "content": chunk["content"],
                        "metadata": chunk.get("metadata", {})
                    }
                )
            )

        self.client.upsert(
            collection_name=settings.COLLECTION_NAME,
            points=points
        )
        return len(points)