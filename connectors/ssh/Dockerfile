# syntax=docker/dockerfile:1
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /ssh-connector ./cmd/ssh-connector

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /ssh-connector /ssh-connector
EXPOSE 8092
ENTRYPOINT ["/ssh-connector"]
