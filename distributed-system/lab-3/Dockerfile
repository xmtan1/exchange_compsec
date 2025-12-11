FROM golang:1.23-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o /bin/chord .

FROM alpine:latest

# 1. Security: Create non-root user
RUN addgroup -S chordgroup && adduser -S chorduser -G chordgroup

# 2. Network: Install CA Certs 
# CRITICAL: Since we added TLS/gRPC, the node needs root certs to verify connections.
RUN apk --no-cache add ca-certificates

WORKDIR /app

# 3. Copy only the binary from the builder stage
COPY --from=builder /bin/chord /app/chord

# COPY --from=builder /src/server-cert.pem /app/
# COPY --from=builder /src/server-key.pem /app/
# COPY --from=builder /src/ca-cert.pem /app/

# Set ownership to the non-root user
RUN chown -R chorduser:chordgroup /app

# Switch user
USER chorduser

# Entrypoint defines the executable
ENTRYPOINT [ "/app/chord" ]

# CMD defines default arguments (can be overridden)
# This serves as a helpful default/documentation
CMD [ "--help" ]