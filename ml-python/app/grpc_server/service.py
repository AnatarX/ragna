import logging
from concurrent import futures
import grpc

from app.ingestion.pipeline import IngestionPipeline
from app.retrieval.hybrid import HybridRetriever
from app.retrieval.reranker import BGEReranker
from app.generation.llm import LLMGenerator
from app.grpc_server.api.v1 import rag_pb2
from app.grpc_server.api.v1 import rag_pb2_grpc

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger("ragna-grpc")

class RagService(rag_pb2_grpc.RagServiceServicer):
    def __init__(self):
        logger.info("Initializing ML components...")
        self.ingestion = IngestionPipeline()
        self.retriever = HybridRetriever()
        self.reranker = BGEReranker()
        self.llm = LLMGenerator()
        logger.info("ML components initialized successfully.")

    def IngestDocument(self, request: rag_pb2.IngestRequest, context) -> rag_pb2.IngestResponse:
        try:
            metadata = dict(request.metadata) if request.metadata else {}
            chunks_count = self.ingestion.process_and_store(
                doc_id=request.document_id,
                content=request.content,
                metadata=metadata
            )
            return rag_pb2.IngestResponse(
                document_id=request.document_id,
                chunks_created=chunks_count,
                success=True
            )
        except Exception as e:
            logger.error(f"Error in IngestDocument: {e}")
            context.set_code(grpc.StatusCode.INTERNAL)
            context.set_details(str(e))
            return rag_pb2.IngestResponse(
                document_id=request.document_id,
                chunks_created=0,
                success=False
            )

    def Query(self, request: rag_pb2.QueryRequest, context) -> rag_pb2.QueryResponse:
        try:
            candidates = self.retriever.dense_search(query=request.query, top_k=20)
            
            if request.use_reranker and candidates:
                top_k = request.top_k if request.top_k > 0 else 5
                final_docs = self.reranker.rerank(query=request.query, candidates=candidates, top_k=top_k)
            else:
                top_k = request.top_k if request.top_k > 0 else 5
                final_docs = candidates[:top_k]

            sources = []
            contexts = []
            for doc in final_docs:
                score = doc.get("rerank_score", doc.get("score", 0.0))
                contexts.append(doc["content"])
                sources.append(
                    rag_pb2.DocumentChunk(
                        id=str(doc["id"]),
                        content=doc["content"],
                        score=float(score),
                        metadata=doc.get("metadata", {})
                    )
                )

            full_answer = "".join(list(self.llm.generate_stream(query=request.query, contexts=contexts)))

            return rag_pb2.QueryResponse(
                answer=full_answer,
                sources=sources,
                confidence_score=sources[0].score if sources else 0.0
            )

        except Exception as e:
            logger.error(f"Error in Query: {e}")
            context.set_code(grpc.StatusCode.INTERNAL)
            context.set_details(str(e))
            return rag_pb2.QueryResponse()

    def StreamQuery(self, request: rag_pb2.QueryRequest, context):
        try:
            candidates = self.retriever.dense_search(query=request.query, top_k=20)
            if request.use_reranker and candidates:
                top_k = request.top_k if request.top_k > 0 else 5
                final_docs = self.reranker.rerank(query=request.query, candidates=candidates, top_k=top_k)
            else:
                top_k = request.top_k if request.top_k > 0 else 5
                final_docs = candidates[:top_k]

            contexts = []
            for doc in final_docs:
                contexts.append(doc["content"])
                chunk_msg = rag_pb2.DocumentChunk(
                    id=str(doc["id"]),
                    content=doc["content"],
                    score=float(doc.get("rerank_score", doc.get("score", 0.0))),
                    metadata=doc.get("metadata", {})
                )
                yield rag_pb2.StreamQueryResponse(source=chunk_msg)

            for token in self.llm.generate_stream(query=request.query, contexts=contexts):
                yield rag_pb2.StreamQueryResponse(delta=token)

        except Exception as e:
            logger.error(f"Error in StreamQuery: {e}")
            context.set_code(grpc.StatusCode.INTERNAL)
            context.set_details(str(e))