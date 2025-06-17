# Stage 1: Build the Go application
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Copy go.mod and go.sum files
COPY go.mod go.sum ./

# Download Go modules
RUN go mod download

# Copy the source code
COPY . .

# Build the application
# Using CGO_ENABLED=0 to build a statically linked binary (recommended for alpine)
# Using -ldflags="-w -s" to strip debug information and reduce binary size (optional)
RUN CGO_ENABLED=0 go build -ldflags="-w -s" -o /app/explorer-server main.go

# Stage 2: Create the final lightweight image
FROM alpine:latest

WORKDIR /app

# Copy the built binary from the builder stage
COPY --from=builder /app/explorer-server /app/explorer-server

# Copy the schema.sql file (for reference or potential use)
COPY schema.sql /app/schema.sql

# Expose the port the application runs on
EXPOSE 8080

# Set the default command to run the application
# The application will respect the PORT environment variable if set, otherwise defaults to 8080 (as coded in main.go)
CMD ["/app/explorer-server"]
