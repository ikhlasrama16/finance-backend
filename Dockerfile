FROM golang:1.26 AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/finance-monitor-api ./cmd/api

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /out/finance-monitor-api /finance-monitor-api
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/finance-monitor-api"]
