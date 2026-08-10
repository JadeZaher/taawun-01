FROM golang:1.21-alpine

WORKDIR /app

# Install dependencies
RUN apk add --no-cache gcc musl-dev git

# Disable checksum verification in Docker build
ENV GOSUMDB=off
ENV GO111MODULE=on

# Copy go mod files
COPY go.mod ./
COPY go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN go build -o main ./cmd/main.go

# Expose port
EXPOSE 8080

# Run the application
CMD ["./main"]
