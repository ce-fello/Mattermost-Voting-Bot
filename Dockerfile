FROM golang:1.24-alpine AS builder
RUN apk add --no-cache gcc musl-dev openssl-dev
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 go build -o /app/myapp

FROM alpine:3.18
RUN apk add --no-cache libssl3
COPY --from=builder /app/myapp /myapp
ENTRYPOINT ["/myapp"]