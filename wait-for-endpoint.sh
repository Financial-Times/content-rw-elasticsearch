#!/bin/bash -e

URL=$1
shift
TIMER_TIMEOUT=$(($1 + 0))
shift
CMD="$@"

echo "Waiting $TIMER_TIMEOUT seconds for $URL to be available..."

TIMER_COUNTER=0
until [ "$(curl -s -o /dev/null -w '%{http_code}' $URL)" == "200" ]; do
  TIMER_COUNTER=$((TIMER_COUNTER + 5))
  if [ $TIMER_COUNTER -gt $TIMER_TIMEOUT ]; then
    >&2 echo "Timeout for $URL exceeded. Exiting."
    exit 1
  fi
  sleep 5
done

>&2 echo "$URL is available. Executing command."

exec $CMD