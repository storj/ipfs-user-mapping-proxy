FROM golang:1.17.5 as build

RUN mkdir /build 

ADD . /build/

WORKDIR /build 

RUN CGO_ENABLED=0 go build

FROM alpine

COPY --from=build /build/ipfs-user-mapping-proxy /app/
COPY --from=build /build/entrypoint /app/

WORKDIR /app

RUN chmod +x ./entrypoint

ENTRYPOINT ["./entrypoint"]