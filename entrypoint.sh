#!/bin/sh

if [ ! -z $PROXY_DEBUG_ADDR ] ; then
  debug_addr="--debug.addr $PROXY_DEBUG_ADDR"
fi

exec ./ipfs-user-mapping-proxy run --address :${PROXY_PORT} --target $PROXY_TARGET --database-url $PROXY_DATABASE_URL $debug_addr
