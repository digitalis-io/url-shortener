FROM golang:1.25-bookworm AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/url-shortener ./cmd/url-shortener

FROM alpine:3.22

RUN apk add --no-cache ca-certificates

WORKDIR /app

COPY --from=build /out/url-shortener /usr/local/bin/url-shortener

EXPOSE 8080

USER 65532:65532

ENTRYPOINT ["/usr/local/bin/url-shortener"]
