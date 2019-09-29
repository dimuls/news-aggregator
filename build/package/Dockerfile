FROM alpine:3 AS builder

RUN apk add --no-cache curl

RUN curl -L http://download.cdn.yandex.net/mystem/mystem-3.1-linux-64bit.tar.gz \
| tar xz



FROM ubuntu:18.04

RUN apt-get update \
&& apt-get install -y --no-install-recommends ca-certificates tzdata \
&& rm -rf /var/lib/apt/lists/*

COPY --from=builder /mystem /mystem
RUN chmod +x /mystem

ENV NEWS_AGGREGATOR_MYSTEM_BIN_PATH=/mystem

COPY news-aggregator /news-aggregator
RUN chmod +x /news-aggregator

ENTRYPOINT ["/news-aggregator"]