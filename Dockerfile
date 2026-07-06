# Stage 1: build
FROM golang:1.22-alpine AS builder
WORKDIR /app
# ENV GOPROXY=https://goproxy.cn,https://proxy.golang.org,direct
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o KyleworksOidcTest .

# Stage 2: run
FROM alpine:3.20
WORKDIR /app
COPY --from=builder /app/KyleworksOidcTest .
COPY templates/ ./templates/
RUN mkdir -p /app/data
EXPOSE 61000
CMD ["./KyleworksOidcTest"]
