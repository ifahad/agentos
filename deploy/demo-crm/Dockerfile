# syntax=docker/dockerfile:1
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /demo-crm ./cmd/demo-crm

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /demo-crm /demo-crm
EXPOSE 8095
ENTRYPOINT ["/demo-crm"]
