# Stage 1: Build the Go binary
FROM golang:1.27-alpine AS builder

WORKDIR /app

# Install dependencies if any
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build a statically linked Go binary named 'bootstrap'
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bootstrap main.go

# Stage 2: Runtime Image
FROM public.ecr.aws/lambda/provided:al2023

# Include AWS Lambda Web Adapter to bridge HTTP requests to your Go server on port 8080
COPY --from=public.ecr.aws/awsguru/aws-lambda-adapter:0.8.4 /lambda-adapter /opt/extensions/lambda-adapter

WORKDIR /var/task

# Copy the compiled Go binary as 'bootstrap'
COPY --from=builder /app/bootstrap /var/task/bootstrap
RUN chmod +x /var/task/bootstrap

# Configure Web Adapter port to match your Go application's port configuration
ENV PORT=8080
ENV AWS_LWA_PORT=8080

CMD ["/var/task/bootstrap"]