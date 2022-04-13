#!/bin/sh

if [ ! -z $PROXY_LOG_FILE ] ; then
  log_file_flag="--log.output $PROXY_LOG_FILE"
fi

if [ -z $PROXY_LOG_LEVEL ] ; then
  PROXY_LOG_LEVEL="info"
fi

if [ ! -z $PROXY_DEBUG_ADDR ] ; then
  debug_addr_flag="--debug.addr $PROXY_DEBUG_ADDR"
fi

exec ./ipfs-user-mapping-proxy run --address :${PROXY_PORT} --target $PROXY_TARGET --database-url $PROXY_DATABASE_URL $log_file_flag --log.level $PROXY_LOG_LEVEL $debug_addr_flag
