# Knowledge Base

This directory stores the local knowledge base used to ground esports answers.

## Layout

- `sources/wikipedia/games/`: seed summaries for core esports game titles
- `sources/wikipedia/teams/`: seed summaries for major organizations
- `sources/wikipedia/players/`: seed summaries for notable players

## Purpose

This seed data is intended to:

1. improve factual grounding for player and team questions
2. serve as initial ingestion input for retrieval and vector indexing
3. provide a reproducible local dataset for the project

## Refresh

Run:

```bash
./scripts/fetch_kb_seed.sh
```

The fetch script downloads public Wikipedia summary JSON pages into the source folders.
