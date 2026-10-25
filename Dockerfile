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
ENTRYPOINT ["gender-api"]
