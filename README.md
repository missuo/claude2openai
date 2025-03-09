# Claude2OpenAI
Used to convert the Claude API to OpenAI compatible API. **Easily use Claude with any OpenAI compatible client.**

## Compatibility
Currently it is only compatible with the Claude-3 family of models, if you pass in any other model, the default will be to use **claude-3-5-haiku-20241022**.

You can customize the list of allowed models by setting the `ALLOWED_MODELS` environment variable with a comma-separated list of model names. This is useful when new Claude models are released, allowing you to add support without rebuilding:

```bash
# Example: Setting custom allowed models
export ALLOWED_MODELS="claude-3-5-haiku-20241022,claude-3-5-sonnet-20241022,claude-3-haiku-20240307"
```

The first model in the list will be used as the default model.

## Request Example
```bash
curl http://127.0.0.1:6600/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer sk-ant-xxxxxxxxxxxxxxxx" \
  -d '{
    "model": "claude-3-5-haiku-20241022",
    "messages": [
      {
        "role": "system",
        "content": "翻译为中文!"
      },
      {
        "role": "user",
        "content": "Hello!"
      }
    ],
    "stream": true
  }'
```

## Usage
### Homebrew (MacOS)

**Special thanks to [Sma1lboy](https://github.com/Sma1lboy) for his contribution.**

```bash
brew tap owo-network/brew
brew install claude2openai
```


### Docker

```bash
docker run -d --restart always -p 6600:6600 ghcr.io/missuo/claude2openai:latest
```

```bash
docker run -d --restart always -p 6600:6600 missuo/claude2openai:latest
```

To set custom allowed models with Docker:

```bash
docker run -d --restart always -p 6600:6600 -e ALLOWED_MODELS="claude-3-5-haiku-20241022,claude-3-5-sonnet-20241022" ghcr.io/missuo/claude2openai:latest
```

### Docker Compose
It is recommended that you use docker version **26.0.0** or higher, otherwise you need to specify the version in the `compose.yaml` file.
```diff
+version: "3.9"
```

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

## License
[MIT](https://github.com/missuo/claude2openai/blob/main/LICENSE)