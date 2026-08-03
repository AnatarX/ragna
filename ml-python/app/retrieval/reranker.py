from sentence_transformers import CrossEncoder
from app.core.config import settings
from typing import List, Dict, Any

class BGEReranker:
    def __init__(self):
        self.model = CrossEncoder(settings.RERANKER_MODEL)

    def rerank(self, query: str, candidates: List[Dict[str, Any]], top_k: int = 5) -> List[Dict[str, Any]]:
        if not candidates:
            return []

        pairs = [[query, doc["content"]] for doc in candidates]
        scores = self.model.predict(pairs)

        for i, doc in enumerate(candidates):
            doc["rerank_score"] = float(scores[i])

        reranked = sorted(candidates, key=lambda x: x["rerank_score"], reverse=True)
        return reranked[:top_k]