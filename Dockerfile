FROM golang:1.20.5 as build

RUN mkdir /build 

ADD . /build/

WORKDIR /build 

RUN CGO_ENABLED=0 go build

FROM alpine

COPY --from=build /build/ipfs-user-mapping-proxy /app/
COPY --from=build /build/entrypoint.sh /app/

WORKDIR /app

RUN chmod +x ./entrypoint.sh

ENTRYPOINT ["./entrypoint.sh"]
