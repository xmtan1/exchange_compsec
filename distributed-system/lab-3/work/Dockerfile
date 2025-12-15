FROM golang:1.25-alpine AS builder

WORKDIR /src

RUN apk add --no-cache openssl

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o /bin/chord .

RUN openssl req -x509 -newkey rsa:4096 -keyout server.key -out server.crt -days 365 -nodes -subj "/CN=localhost"

FROM alpine:latest

# 1. Security: Create non-root user
RUN addgroup -S chordgroup && adduser -S chorduser -G chordgroup

# 2. Network: Install CA Certs 
# CRITICAL: Since we added TLS/gRPC, the node needs root certs to verify connections.
RUN apk --no-cache add ca-certificates

WORKDIR /app

# 3. Copy only the binary from the builder stage
COPY --from=builder /bin/chord /app/chord

COPY --from=builder /src/server.crt /app/
COPY --from=builder /src/server.key /app/

# COPY --from=builder /src/server-cert.pem /app/
# COPY --from=builder /src/server-key.pem /app/
# COPY --from=builder /src/ca-cert.pem /app/

# Set ownership to the non-root user
RUN chown -R chorduser:chordgroup /app

# Switch user
USER chorduser

# Expose ports
EXPOSE 3410 4170

# Entrypoint defines the executable
ENTRYPOINT [ "/app/chord" ]

# CMD defines default arguments (can be overridden)
# This serves as a helpful default/documentation
CMD [ "--help" ]