FROM golang:1.19 as builder

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY app/. .
RUN CGO_ENABLED=0 go build -o server main.go

FROM alpine:3.16
WORKDIR /app
COPY --from=builder /build/server ./
ENTRYPOINT ["/app/server"]
EXPOSE 8000/tcp
