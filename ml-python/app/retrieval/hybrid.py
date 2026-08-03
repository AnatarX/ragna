from qdrant_client import QdrantClient
from fastembed import TextEmbedding
from app.core.config import settings
from typing import List, Dict, Any

class HybridRetriever:
    def __init__(self):
        self.client = QdrantClient(host=settings.QDRANT_HOST, port=settings.QDRANT_PORT)
        self.embed_model = TextEmbedding(model_name=settings.EMBEDDING_MODEL)

    def dense_search(self, query: str, top_k: int = 20) -> List[Dict[str, Any]]:
        query_vector = list(self.embed_model.embed([query]))[0].tolist()
        
        search_result = self.client.search(
            collection_name=settings.COLLECTION_NAME,
            query_vector=query_vector,
            limit=top_k
        )
        
        results = []
        for hit in search_result:
            results.append({
                "id": hit.id,
                "content": hit.payload.get("content", ""),
                "score": hit.score,
                "metadata": hit.payload.get("metadata", {})
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