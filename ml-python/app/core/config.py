import os
from pydantic_settings import BaseSettings

import os
from pydantic_settings import BaseSettings

class Settings(BaseSettings):
    PROJECT_NAME: str = "ragna-ml"
    
    QDRANT_HOST: str = os.getenv("QDRANT_HOST", "qdrant")
    QDRANT_PORT: int = int(os.getenv("QDRANT_PORT", 6333))
    COLLECTION_NAME: str = "ragna_docs"
    
    EMBEDDING_MODEL: str = "BAAI/bge-small-en-v1.5"
    VECTOR_SIZE: int = 384
    RERANKER_MODEL: str = "BAAI/bge-reranker-base"
    
    LLM_PROVIDER: str = os.getenv("LLM_PROVIDER", "ollama") # "ollama" или "openai"
    OLLAMA_BASE_URL: str = os.getenv("OLLAMA_BASE_URL", "http://host.docker.internal:11434")
    OLLAMA_MODEL: str = os.getenv("OLLAMA_MODEL", "llama3.2")
    
    OPENAI_API_KEY: str = os.getenv("OPENAI_API_KEY", "")
    OPENAI_MODEL: str = os.getenv("OPENAI_MODEL", "gpt-4o-mini")

    class Config:
        env_file = ".env"

settings = Settings()