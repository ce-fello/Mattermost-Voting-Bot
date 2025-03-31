ARG PLATFORM=linux/amd64
FROM golang:1.24-alpine AS builder

RUN apk add --no-cache gcc musl-dev openssl-dev git

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 go build -ldflags="-s -w" -o /app/myapp

FROM --platform=$PLATFORM alpine:3.18
RUN apk add --no-cache libssl3 ca-certificates
COPY --from=builder /app/myapp /myapp
ENTRYPOINT ["/myapp"]