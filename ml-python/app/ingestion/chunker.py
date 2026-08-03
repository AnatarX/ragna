from typing import List, Dict, Any
import uuid

class TextChunker:
    def __init__(self, chunk_size: int = 500, overlap: int = 50):
        self.chunk_size = chunk_size
        self.overlap = overlap

    def split_text(self, text: str, doc_id: str, metadata: Dict[str, Any] = None) -> List[Dict[str, Any]]:
        paragraphs = text.split("\n\n")
        chunks = []
        current_chunk = ""
        
        for para in paragraphs:
            para = para.strip()
            if not para:
                continue
                
            if len(current_chunk) + len(para) <= self.chunk_size:
                current_chunk += ("\n\n" if current_chunk else "") + para
            else:
                if current_chunk:
                    chunks.append(self._build_chunk_payload(current_chunk, doc_id, metadata))
                overlap_text = current_chunk[-self.overlap:] if len(current_chunk) > self.overlap else ""
                current_chunk = (overlap_text + "\n\n" + para).strip()
                
        if current_chunk:
            chunks.append(self._build_chunk_payload(current_chunk, doc_id, metadata))
            
        return chunks

    def _build_chunk_payload(self, content: str, doc_id: str, metadata: Dict[str, Any] = None) -> Dict[str, Any]:
        payload = {
            "chunk_id": str(uuid.uuid4()),
            "doc_id": doc_id,
            "content": content,
        }
        if metadata:
            payload["metadata"] = metadata
        return payload