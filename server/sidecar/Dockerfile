FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o sidecar .

FROM alpine:latest
RUN apk --no-cache add ca-certificates
RUN addgroup -g 1001 -S appgroup && adduser -u 1001 -S appuser -G appgroup
WORKDIR /app
COPY --from=builder /app/sidecar .
RUN chown -R appuser:appgroup /app
USER appuser
EXPOSE 9000
CMD ["./sidecar"]
