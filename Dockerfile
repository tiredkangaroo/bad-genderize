# syntax=docker/dockerfile:1

FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/gender-api .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=build /out/gender-api /usr/local/bin/gender-api
USER nobody
EXPOSE 8081
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget -qO- http://127.0.0.1:8081/healthz >/dev/null || exit 1
ENTRYPOINT ["gender-api"]
