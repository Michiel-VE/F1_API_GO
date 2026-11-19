# Stage 1: Build the Go binary
FROM golang:1.27.4-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build the whole main package (not just main.go) as a static binary named 'bootstrap'
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o bootstrap .

# Stage 2: Runtime Image
FROM public.ecr.aws/lambda/provided:al2023

# AWS Lambda Web Adapter bridges Lambda events to the Go HTTP server on port 8080
COPY --from=public.ecr.aws/awsguru/aws-lambda-adapter:0.8.4 /lambda-adapter /opt/extensions/lambda-adapter
RUN chmod 0755 /opt/extensions/lambda-adapter

WORKDIR /var/task

COPY --from=builder /app/bootstrap /var/task/bootstrap
RUN chmod 0755 /var/task/bootstrap

ENV PORT=8080
ENV AWS_LWA_PORT=8080

# Run as a non-root user (also satisfies container static analysis checks)
RUN useradd --system --no-create-home --shell /sbin/nologin appuser
USER appuser

CMD ["/var/task/bootstrap"]