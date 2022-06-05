FROM golang:1.16-bullseye AS builder

ARG VERSION=0.0.0

ENV GOBIN /go/bin
WORKDIR /go/src/

COPY . .

RUN echo $VERSION > /go/src/VERSION
# RUN go env -w GOFLAGS=-mod=mod && go run github.com/99designs/gqlgen
RUN go get .

RUN CGO_ENABLED=0 GOOS=linux go build -o /go/bin/core .

FROM alpine:latest

WORKDIR /app
COPY --from=builder /go/bin/core /go/src/VERSION ./
COPY --from=builder /go/src/views ./views
COPY --from=builder /go/src/assets ./assets
EXPOSE 3000

CMD [ "/app/core" ]
