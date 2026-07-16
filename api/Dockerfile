FROM golang:1.26.3-alpine AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/rag-api .

FROM alpine:3.23

RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=builder /out/rag-api ./rag-api
COPY etc ./etc

EXPOSE 9999
USER 65532:65532
ENTRYPOINT ["./rag-api"]
