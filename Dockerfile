FROM golang:1.16-bullseye AS builder

ARG VERSION=0.0.0

ENV GOBIN /go/bin
WORKDIR /go/src/

COPY . .

RUN echo $VERSION > /go/src/VERSION
# RUN go env -w GOFLAGS=-mod=mod && go run github.com/99designs/gqlgen
RUN go get .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=386 go build -o /go/bin/ouno .

FROM alpine:latest

WORKDIR /app
COPY --from=builder /go/bin/ouno /go/src/VERSION ./
COPY --from=builder /go/src/views ./views
COPY --from=builder /go/src/assets ./assets
EXPOSE 3000

CMD [ "/app/ouno" ]
