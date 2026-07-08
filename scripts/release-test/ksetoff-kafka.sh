#!/usr/bin/env sh

set -eu

script_dir="$(CDPATH= cd "$(dirname "$0")" && pwd)"
version="${VERSION:?set VERSION to the release tag, for example v0.3.0}"
kafka_image="${KAFKA_IMAGE:-apache/kafka:4.3.1}"
ubuntu_image="${IMAGE:-ubuntu:24.04}"
net="${NET:-toolbox-ksetoff-test-net}"
vol="${VOL:-toolbox-ksetoff-test-tools}"
broker="${BROKER:-toolbox-ksetoff-test-broker}"
topic="${TOPIC:-toolbox-ksetoff-release}"
group="${GROUP:-toolbox-ksetoff-group}"
subset_group="${SUBSET_GROUP:-toolbox-ksetoff-subset-group}"

cleanup() {
	docker rm -f "$broker" >/dev/null 2>&1 || true
	docker volume rm "$vol" >/dev/null 2>&1 || true
	docker network rm "$net" >/dev/null 2>&1 || true
}

trap cleanup EXIT INT TERM
cleanup

docker network create "$net" >/dev/null
docker volume create "$vol" >/dev/null

docker run --rm -d \
	--name "$broker" \
	--network "$net" \
	-v "$vol:/tools" \
	"$kafka_image" >/dev/null

for i in $(seq 1 60); do
	if docker exec "$broker" /opt/kafka/bin/kafka-topics.sh \
		--bootstrap-server localhost:9092 \
		--list >/dev/null 2>&1; then
		break
	fi
	if [ "$i" -eq 60 ]; then
		docker logs --tail 80 "$broker"
		exit 1
	fi
	sleep 2
done

docker run --rm \
	-e VERSION="$version" \
	-e TOOLBOX_BIN=/tools \
	-v "$vol:/tools" \
	-v "$script_dir:/release-test:ro" \
	"$ubuntu_image" \
	sh -lc '
set -eu
export DEBIAN_FRONTEND=noninteractive
apt-get update -qq >/dev/null
apt-get install -y -qq curl ca-certificates >/dev/null
sh /release-test/install-tool.sh ksetoff
'

docker exec "$broker" sh -lc 'printf "bootstrap.servers=localhost:9092\n" > /tmp/kafka.conf'

docker exec "$broker" /opt/kafka/bin/kafka-topics.sh \
	--bootstrap-server localhost:9092 \
	--create \
	--if-not-exists \
	--topic "$topic" \
	--partitions 2 \
	--replication-factor 1 >/dev/null

seq 1 10 | docker exec -i "$broker" /opt/kafka/bin/kafka-console-producer.sh \
	--bootstrap-server localhost:9092 \
	--topic "$topic" >/dev/null

sleep 2

docker exec "$broker" /tools/ksetoff \
	-F /tmp/kafka.conf \
	-group "$group" \
	-topic "$topic" \
	-offset earliest \
	-dry-run | grep -q "$group"

docker exec "$broker" /opt/kafka/bin/kafka-consumer-groups.sh \
	--bootstrap-server localhost:9092 \
	--describe \
	--group "$group" 2>&1 | grep -q "does not exist" || {
	printf 'FAIL ksetoff dry-run created or moved group\n' >&2
	exit 1
}

docker exec "$broker" /tools/ksetoff \
	-F /tmp/kafka.conf \
	-group "$group" \
	-topic "$topic" \
	-offset latest >/dev/null

docker exec "$broker" /opt/kafka/bin/kafka-consumer-groups.sh \
	--bootstrap-server localhost:9092 \
	--describe \
	--group "$group" > /tmp/ksetoff-latest-group.out
grep -Eq "${topic}[[:space:]]+0[[:space:]]+[0-9]+[[:space:]]+[0-9]+[[:space:]]+0" /tmp/ksetoff-latest-group.out
grep -Eq "${topic}[[:space:]]+1[[:space:]]+[0-9]+[[:space:]]+[0-9]+[[:space:]]+0" /tmp/ksetoff-latest-group.out

docker exec "$broker" /tools/ksetoff \
	-F /tmp/kafka.conf \
	-group "$group" \
	-topic "$topic" \
	-offset 2 >/dev/null

docker exec "$broker" /opt/kafka/bin/kafka-consumer-groups.sh \
	--bootstrap-server localhost:9092 \
	--describe \
	--group "$group" > /tmp/ksetoff-numeric-group.out
grep -Eq "${topic}[[:space:]]+0[[:space:]]+2[[:space:]]+[0-9]+" /tmp/ksetoff-numeric-group.out
grep -Eq "${topic}[[:space:]]+1[[:space:]]+2[[:space:]]+[0-9]+" /tmp/ksetoff-numeric-group.out

docker exec "$broker" /tools/ksetoff \
	-F /tmp/kafka.conf \
	-group "$group" \
	-topic "$topic" \
	-offset beginning >/dev/null

docker exec "$broker" /opt/kafka/bin/kafka-consumer-groups.sh \
	--bootstrap-server localhost:9092 \
	--describe \
	--group "$group" > /tmp/ksetoff-earliest-group.out
grep -Eq "${topic}[[:space:]]+0[[:space:]]+0[[:space:]]+[0-9]+" /tmp/ksetoff-earliest-group.out
grep -Eq "${topic}[[:space:]]+1[[:space:]]+0[[:space:]]+[0-9]+" /tmp/ksetoff-earliest-group.out

docker exec "$broker" /tools/ksetoff \
	-F /tmp/kafka.conf \
	-group "$group" \
	-topic "$topic" \
	-offset "timestamp:1970-01-01T00:00:00Z" >/dev/null

docker exec "$broker" /opt/kafka/bin/kafka-consumer-groups.sh \
	--bootstrap-server localhost:9092 \
	--describe \
	--group "$group" > /tmp/ksetoff-timestamp-group.out
grep -Eq "${topic}[[:space:]]+0[[:space:]]+0[[:space:]]+[0-9]+" /tmp/ksetoff-timestamp-group.out
grep -Eq "${topic}[[:space:]]+1[[:space:]]+0[[:space:]]+[0-9]+" /tmp/ksetoff-timestamp-group.out

docker exec "$broker" /tools/ksetoff \
	-F /tmp/kafka.conf \
	-group "$subset_group" \
	-topic "$topic" \
	-offset end \
	-partitions 1 >/dev/null

docker exec "$broker" /opt/kafka/bin/kafka-consumer-groups.sh \
	--bootstrap-server localhost:9092 \
	--describe \
	--group "$subset_group" > /tmp/ksetoff-subset-group.out
grep -Eq "${topic}[[:space:]]+1[[:space:]]+[0-9]+[[:space:]]+[0-9]+[[:space:]]+0" /tmp/ksetoff-subset-group.out
if grep -Eq "${topic}[[:space:]]+0[[:space:]]+" /tmp/ksetoff-subset-group.out; then
	printf 'FAIL ksetoff partition subset moved partition 0\n' >&2
	exit 1
fi

printf 'KSETOFF KAFKA RELEASE TEST PASSED\n'
