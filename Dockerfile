FROM golang:1.20-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go env -w GOPROXY=https://proxy.golang.org,direct
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /go-student-service ./cmd/service

FROM alpine:3.18
RUN apk add --no-cache ca-certificates
COPY --from=builder /go-student-service /usr/local/bin/go-student-service
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/go-student-service"]
