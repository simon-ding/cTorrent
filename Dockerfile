FROM golang:1.13-alpine AS build_base

RUN apk add alpine-sdk
WORKDIR /torrent/

ENV GO111MODULE=on

COPY go.mod .
COPY go.sum .

RUN go mod download

FROM build_base AS server_builder
COPY . .
RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build  -a -ldflags \
    "-extldflags '-static'" -o ./cmd/cloud_torrent .

FROM alpine AS server
WORKDIR /torrent/
COPY --from=server_builder /torrent/cmd/cloud_torrent .
COPY ./static ./static

ENTRYPOINT ["/torrent/cloud_torrent"]