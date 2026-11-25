# 負荷試験用の API image。build context は api/ で、.env などを image に入れないよう必要なディレクトリだけをコピーする。
FROM golang:1.26.10-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
COPY docs ./docs
RUN CGO_ENABLED=0 go build -o /out/threadly-api ./cmd/api

FROM alpine:3.22
COPY --from=build /out/threadly-api /usr/local/bin/threadly-api
USER nobody
ENTRYPOINT ["threadly-api"]
