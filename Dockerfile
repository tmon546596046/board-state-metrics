FROM golang:1.16.3 AS builder
WORKDIR /go/src/github.com/tmon546596046/board-state-metrics
COPY . .
RUN make build

FROM alpine:3.7
LABEL io.k8s.display-name="board-state-metrics" \
      io.k8s.description="This is a component that exposes metrics about Board objects." \


ARG FROM_DIRECTORY=/go/src/github.com/tmon546596046/board-state-metrics
COPY --from=builder ${FROM_DIRECTORY}/board-state-metrics  /usr/bin/board-state-metrics

USER nobody
ENTRYPOINT ["/usr/bin/board-state-metrics"]
