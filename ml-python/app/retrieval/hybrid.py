from qdrant_client import QdrantClient
from qdrant_client.http import models
from fastembed import TextEmbedding
from app.core.config import settings
from typing import List, Dict, Any

class HybridRetriever:
    def __init__(self):
        self.client = QdrantClient(host=settings.QDRANT_HOST, port=settings.QDRANT_PORT)
        self.encoder = TextEmbedding(model_name=settings.EMBEDDING_MODEL)
        self._ensure_collection()

    def _ensure_collection(self):
        collections = [c.name for c in self.client.get_collections().collections]
        if settings.COLLECTION_NAME not in collections:
            self.client.create_collection(
                collection_name=settings.COLLECTION_NAME,
                vectors_config=models.VectorParams(
                    size=settings.VECTOR_SIZE,
                    distance=models.Distance.COSINE
                )
            )
    def dense_search(self, query: str, top_k: int = 5):
        return self.search(query=query, top_k=top_k)
    
    def search(self, query: str, top_k: int = 5):
        query_vector = list(self.encoder.embed([query]))[0]

        response = self.client.query_points(
            collection_name=settings.COLLECTION_NAME,
            query=query_vector.tolist(),
            limit=top_k
        )

        results = []
        for point in response.points:
            results.append({
                "id": str(point.id),
                "score": point.score,
                "content": point.payload.get("content", ""),
                "metadata": point.payload.get("metadata", {})
            })

        return results

    def rrf_score(self, dense_results: List[Dict[str, Any]], k: int = 60) -> List[Dict[str, Any]]:
        rrf_map = {}
        
        for rank, doc in enumerate(dense_results):
            doc_id = doc["id"]
            if doc_id not in rrf_map:
                rrf_map[doc_id] = {"doc": doc, "score": 0.0}
            rrf_map[doc_id]["score"] += 1.0 / (k + rank + 1)
            
        sorted_docs = sorted(rrf_map.values(), key=lambda x: x["score"], reverse=True)
        
        res = []
        for item in sorted_docs:
            d = item["doc"]
            d["rrf_score"] = item["score"]
            res.append(d)
        return res