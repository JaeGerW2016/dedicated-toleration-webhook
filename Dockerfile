FROM golang:1.12-buster as builder

ENV GOPROXY https://goproxy.io
ENV GO111MODULE on

WORKDIR /go/cache
COPY [ "go.mod","go.sum","./"]
RUN go mod download

WORKDIR /go/release
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o dedicated-toleration-webhook .

FROM 314315960/alpine-base:3.9
WORKDIR /
COPY --from=builder /go/release/dedicated-toleration-webhook .
ENTRYPOINT ["/dedicated-toleration-webhook"]