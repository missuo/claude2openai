FROM golang:1.22 AS builder
WORKDIR /go/src/github.com/missuo/claude2openai
COPY main.go ./
COPY go.mod ./
COPY go.sum ./
COPY types.go ./
RUN go get -d -v ./
RUN CGO_ENABLED=0 go build -a -installsuffix cgo -o claude2openai .

FROM alpine:latest
WORKDIR /app
COPY --from=builder /go/src/github.com/missuo/claude2openai/claude2openai /app/claude2openai
CMD ["/app/claude2openai"]