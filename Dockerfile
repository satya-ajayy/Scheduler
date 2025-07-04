# base image
FROM golang:1.23.3-alpine as base
WORKDIR /scheduler

ENV CGO_ENABLED=0

COPY go.mod go.sum /scheduler/
RUN go mod download

ADD . .
RUN go build -o /usr/local/bin/scheduler ./cmd/scheduler

# runner image with shell (alpine)
FROM alpine:latest
RUN apk add --no-cache tzdata curl

WORKDIR /app
COPY --from=base /usr/local/bin/scheduler scheduler

EXPOSE 4202
ENTRYPOINT ["/app/scheduler"]
