# syntax=docker/dockerfile:1
FROM golang:1.27.1-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/library ./cmd/library

FROM alpine:3.23
RUN apk add --no-cache ca-certificates tzdata && addgroup -S library && adduser -S -G library library
WORKDIR /app
COPY --from=build /out/library /app/library
USER library:library
EXPOSE 8080
HEALTHCHECK --interval=10s --timeout=5s --start-period=40s --retries=3 CMD ["/app/library", "healthcheck"]
ENTRYPOINT ["/app/library"]
CMD ["serve"]
