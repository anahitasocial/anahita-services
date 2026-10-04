#!/bin/sh
# Makes the certificates the services use to talk to each other over gRPC in
# docker-compose: a private certificate authority, and one certificate signed
# by it that every service presents, as server and as caller.
#
# Safe to run every time: it does nothing while the certificate has more than
# 30 days left. `make up` and `make up_build` run it before the containers
# start, because they mount ./grpc-certs. It can also be run by hand.
#
# For local development only. On Kubernetes cert-manager issues a certificate
# per service: see k8s/base/grpc-tls.yml.

set -eu

DIR="${GRPC_CERTS_DIR:-./grpc-certs}"
SERVICES="graph-grpc-service file-grpc-service mail-grpc-service"

if [ -f "$DIR/tls.crt" ] && [ -f "$DIR/tls.key" ] && [ -f "$DIR/ca.crt" ] \
    && openssl x509 -in "$DIR/tls.crt" -noout -checkend 2592000 >/dev/null 2>&1; then
    exit 0
fi

echo "grpc-certs-init: making development certificates in $DIR"
mkdir -p "$DIR"

# The authority. Its key is deleted once the certificate is signed: the whole
# directory is mounted into the containers, and none of them should be able to
# sign a certificate of its own.
openssl req -x509 -newkey ec -pkeyopt ec_paramgen_curve:P-256 -nodes \
    -keyout "$DIR/ca.key" -out "$DIR/ca.crt" -days 3650 \
    -subj "/CN=Anahita development gRPC authority" >/dev/null 2>&1

names="DNS:localhost"
for service in $SERVICES; do
    names="$names,DNS:$service"
done

cat > "$DIR/tls.ext" <<EXT
basicConstraints = CA:FALSE
keyUsage = digitalSignature
extendedKeyUsage = serverAuth, clientAuth
subjectAltName = $names
EXT

openssl req -new -newkey ec -pkeyopt ec_paramgen_curve:P-256 -nodes \
    -keyout "$DIR/tls.key" -out "$DIR/tls.csr" \
    -subj "/CN=anahita-dev" >/dev/null 2>&1

openssl x509 -req -in "$DIR/tls.csr" -CA "$DIR/ca.crt" -CAkey "$DIR/ca.key" \
    -CAcreateserial -out "$DIR/tls.crt" -days 365 -extfile "$DIR/tls.ext" >/dev/null 2>&1

rm -f "$DIR/tls.csr" "$DIR/tls.ext" "$DIR/ca.srl" "$DIR/ca.key"

# The containers run as root and read these through a read-only mount.
chmod 644 "$DIR/tls.crt" "$DIR/ca.crt" "$DIR/tls.key"

echo "grpc-certs-init: done. Restart the services if they were already running."
