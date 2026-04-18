FROM golang:1.23 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /usr/local/bin/worker .

FROM gcr.io/distroless/static:nonroot

COPY --from=builder /usr/local/bin/worker /usr/local/bin/worker

EXPOSE 9090

ENTRYPOINT ["/usr/local/bin/worker"]
