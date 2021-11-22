FROM golang:1.17-alpine AS builder

RUN apk add upx

WORKDIR /go/src/github.com/slntopp/nocloud-driver-ione
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" .
RUN upx ./nocloud-driver-ione

FROM scratch
WORKDIR /
COPY --from=builder  /go/src/github.com/slntopp/nocloud-driver-ione/nocloud-driver-ione /nocloud-driver-ione

EXPOSE 8080
LABEL org.opencontainers.image.source https://github.com/slntopp/nocloud-driver-ione

ENTRYPOINT ["/nocloud-driver-ione"]
