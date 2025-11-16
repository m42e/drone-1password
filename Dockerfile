FROM golang:1.21-alpine AS build
WORKDIR /src

RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY plugin ./plugin
COPY main.go ./

RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /bin/drone-secret-1password

FROM alpine:3.19
RUN apk add --no-cache ca-certificates

ENV GODEBUG=netdns=go
EXPOSE 3000

COPY --from=build /bin/drone-secret-1password /bin/drone-secret-1password

ENTRYPOINT ["/bin/drone-secret-1password"]
