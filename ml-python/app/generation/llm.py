import json
import httpx
from typing import List, Generator
from openai import OpenAI
from app.core.config import settings

SYSTEM_PROMPT = """You are a helpful and accurate AI assistant. 
Answer the user's question using ONLY the provided context snippets below.
If the answer cannot be derived from the context, say "I don't have enough information in my knowledge base to answer this."

Context snippets:
{context}
"""

class LLMGenerator:
    def __init__(self):
        self.provider = settings.LLM_PROVIDER
        if self.provider == "openai":
            self.client = OpenAI(api_key=settings.OPENAI_API_KEY)

    def generate_stream(self, query: str, contexts: List[str]) -> Generator[str, None, None]:
        context_str = "\n---\n".join(contexts)
        system_msg = SYSTEM_PROMPT.format(context=context_str)

        if self.provider == "ollama":
            yield from self._stream_ollama(system_msg, query)
        elif self.provider == "openai":
            yield from self._stream_openai(system_msg, query)
        else:
            yield f"Error: Unknown LLM provider '{self.provider}'"

    def _stream_ollama(self, system_msg: str, query: str) -> Generator[str, None, None]:
        url = f"{settings.OLLAMA_BASE_URL}/api/generate"
        payload = {
            "model": settings.OLLAMA_MODEL,
            "system": system_msg,
            "prompt": query,
            "stream": True
        }
        
        try:
            with httpx.stream("POST", url, json=payload, timeout=60.0) as response:
                for line in response.iter_lines():
                    if line:
                        data = json.loads(line)
                        delta = data.get("response", "")
                        if delta:
                            yield delta
        except Exception as e:
            yield f"\n[LLM Error: Could not connect to Ollama at {settings.OLLAMA_BASE_URL}: {e}]"

    def _stream_openai(self, system_msg: str, query: str) -> Generator[str, None, None]:
        try:
            response = self.client.chat.completions.create(
                model=settings.OPENAI_MODEL,
                messages=[
                    {"role": "system", "content": system_msg},
                    {"role": "user", "content": query}
                ],
                stream=True
            )
            for chunk in response:
                delta = chunk.choices[0].delta.content or ""
                if delta:
                    yield delta
        except Exception as e:
            yield f"\n[OpenAI Error: {e}]"