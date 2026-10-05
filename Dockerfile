FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /out/pushotpd ./cmd/pushotpd

FROM alpine:3.20
RUN apk add --no-cache ca-certificates wget
COPY --from=build /out/pushotpd /pushotpd
EXPOSE 8080
ENTRYPOINT ["/pushotpd"]
CMD ["-config", "/etc/pushotp/config.yaml"]
