# Claude2OpenAI Development Guide

## Build & Run Commands
```bash
# Build the application
go build -o claude2openai .

# Run locally (default port: 6600)
./claude2openai

# Run with custom port
./claude2openai -p 8080

# Docker build
docker build -t claude2openai .

# Docker run
docker run -d --restart always -p 6600:6600 claude2openai
```

## Code Style Guidelines
- **Naming**: Use camelCase for functions and variables
- **Error Handling**: Return JSON responses with descriptive error messages and appropriate HTTP status codes
- **Comments**: Add comments for non-obvious code sections, consider bilingual (English/Chinese) for key components
- **Imports**: Group standard library imports separate from external dependencies
- **Formatting**: Maintain consistent struct definitions with appropriate JSON tags
- **Types**: Follow Go idioms with proper type definitions in types.go
- **Structure**: Maintain separation between OpenAI and Claude data structures

This project is a proxy service that converts Anthropic's Claude API to an OpenAI-compatible format.