FROM golang:1.19 as builder

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY auth/. .
RUN CGO_ENABLED=0 go build -o auth main.go

FROM alpine:3.16
WORKDIR /app
COPY --from=builder /build/auth ./
ENTRYPOINT ["/app/auth"]
EXPOSE 8000/tcp
