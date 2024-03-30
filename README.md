# Claude2OpenAI
This project is used to convert the Claude API to OpenAI compatible API.

## Usage
### Docker

```bash
docker run -d --restart always -p 6060:6060 ghcr.io/missuo/claude2openai:latest
```

```bash
docker run -d --restart always -p 6060:6060 missuo/claude2openai:latest
```

### Docker Compose

```bash
mkdir claude2openai && cd claude2openai
wget -O compose.yaml https://raw.githubusercontent.com/missuo/claude2openai/main/compose.yaml
docker compose up -d
```

### Manual

Download the latest release from the [release page](https://github.com/missuo/claude2openai/releases).

```bash
chmod +x claude2openai
./claude2openai
```