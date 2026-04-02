FROM golang:latest AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /broker ./broker

FROM alpine:3
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=builder /broker /broker
ENTRYPOINT ["/broker"]