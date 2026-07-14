FROM golang:1.27.1-alpine AS build
RUN apk add --no-cache gcc musl-dev
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY core ./core
COPY protocol ./protocol
COPY cmd ./cmd
RUN CGO_ENABLED=1 go build -trimpath -o /nsf ./cmd/nsf

FROM alpine:3.22
RUN apk add --no-cache git ca-certificates && adduser -D -u 10001 nsf && mkdir /state && chown nsf /state
COPY --from=build /nsf /usr/local/bin/nsf
USER nsf
WORKDIR /app
ENTRYPOINT ["nsf"]
CMD ["-config", "/app/config.json", "-listen", "0.0.0.0:8787", "serve"]
