# Esports Arena AI

Open-source eSports chatbot built with Go, Ollama, and a ChatGPT-style frontend.

## What it does

- Uses a local generation model through Ollama
- Uses local Ollama embeddings plus Qdrant for RAG
- Answers only about eSports and competitive gaming
- Refuses out-of-scope prompts
- Blocks common prompt injection attempts
- Refuses toxic or abusive inputs
- Tries to answer in the same language used by the user
- Ships with a frontend inspired by ChatGPT

## Stack

- Go standard library for the backend
- Ollama for local open-source model serving
- Local generation model via Ollama
- `nomic-embed-text` via Ollama for embeddings
- Qdrant as the vector database
- Plain HTML, CSS, and JavaScript for the frontend
- Redis for conversation persistence and response caching
- Docker Compose for local orchestration

## Quick start

1. Install Ollama and pull the generation and embedding models:

```bash
ollama pull mistral
ollama pull nomic-embed-text
```

2. Copy the environment file:

```bash
cp .env.example .env
```

3. Run the Go app:

```bash
go run ./cmd/server
```

4. Ingest the local knowledge base into Qdrant:

```bash
go run ./cmd/ingest
```

5. Open the app:

```text
http://localhost:8080
```

If you also want persisted recent conversations and backend caching locally, start Redis too:

```bash
docker run --name esports-chatbot-redis -p 6379:6379 redis:7-alpine
```

Also start Qdrant locally:

```bash
docker run --name esports-chatbot-qdrant -p 6333:6333 qdrant/qdrant
```

## Docker

Start the full stack:

```bash
docker compose up --build
```

Then pull the models inside the Ollama container:

```bash
docker exec -it esports-chatbot-ollama ollama pull mistral
docker exec -it esports-chatbot-ollama ollama pull nomic-embed-text
```

And ingest the knowledge base:

```bash
docker exec -it esports-chatbot-app sh -lc 'cd /app && go run ./cmd/ingest'
```

This setup runs 4 containers:

- `app` for the Go backend and embedded frontend
- `ollama` for the local open-source model
- `redis` for recent conversations and cache
- `qdrant` for vector search

## API

### `GET /health`

Returns backend status and the configured model.

### `GET /api/conversations`

Returns recent conversations for the sidebar.

### `GET /api/conversations/:id`

Returns the full message history for a saved conversation.

### `POST /api/ingest`

Loads the local knowledge base, generates embeddings through Ollama, and indexes the documents into Qdrant.

### `POST /api/chat`

Request body:

```json
{
  "message": "Who is Faker in League of Legends?",
  "conversation_id": "conv_123",
  "history": [
    { "role": "user", "content": "What happened at Worlds?" }
  ]
}
```

## RAG flow

1. Seed and curated esports documents live in `knowledge_base/`
2. `go run ./cmd/ingest` loads those files
3. Ollama generates embeddings using `nomic-embed-text`
4. Qdrant stores the vectors
5. Chat queries retrieve relevant context before answer generation

Response body:

```json
{
  "answer": "Faker is the mid laner for T1...",
  "language": "en",
  "blocked": false,
  "model": "llama3.2:1b",
  "domain_allowed": true,
  "conversation_id": "conv_123",
  "cached": false
}
```

## Guardrails design

The app uses layered guardrails before generation:

1. Rule-based detection for toxic or abusive input
2. Rule-based detection for prompt injection and jailbreak patterns
3. eSports domain filtering based on keywords and recent chat context
4. Fallback topic classification with the local model when the domain is ambiguous
5. System prompting that repeats domain limits and multilingual response behavior

Redis is used for:

1. Persisting recent conversations for the sidebar
2. Reopening full message history
3. Caching repeated chat requests for faster answers

If Redis is unavailable, the app falls back to in-memory storage automatically.
