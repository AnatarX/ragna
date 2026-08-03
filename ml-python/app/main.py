import os
import time
from concurrent import futures
import grpc

from app.grpc_server.service import RagService
from app.grpc_server.api.v1 import rag_pb2_grpc

def serve():
    port = os.getenv("GRPC_PORT", "50051")
    server = grpc.server(futures.ThreadPoolExecutor(max_workers=10))
    rag_pb2_grpc.add_RagServiceServicer_to_server(RagService(), server)
    
    server.add_insecure_port(f"[::]:{port}")
    server.start()
    print(f"gRPC Server running on port {port}")
    
    try:
        while True:
            time.sleep(86400)
    except KeyboardInterrupt:
        server.stop(0)

if __name__ == "__main__":
    serve()