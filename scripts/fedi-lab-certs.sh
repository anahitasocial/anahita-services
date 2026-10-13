#!/bin/sh
# Makes the certificates for the federation lab (the `fedi-lab` profile in
# docker-compose.yml): a private certificate authority, and one certificate
# signed by it for the three names the lab answers to.
#
# Other servers are only spoken to over https, at a name, so trying
# federation on one machine needs both. The names end in .test, which is
# kept for exactly this and can never be a real site.
#
# Safe to run every time: it does nothing while the certificate has more than
# 30 days left. `make fedi_lab` runs it. For local development only.

set -eu

DIR="${FEDI_LAB_CERTS_DIR:-./fedi-lab/certs}"
NAMES="anahita.test api.anahita.test gts.test"

if [ -f "$DIR/tls.crt" ] && [ -f "$DIR/tls.key" ] && [ -f "$DIR/ca.crt" ] \
    && openssl x509 -in "$DIR/tls.crt" -noout -checkend 2592000 >/dev/null 2>&1; then
    exit 0
fi

echo "fedi-lab-certs: making the lab's certificates in $DIR"
mkdir -p "$DIR"

# The authority. Its key is deleted once the certificate is signed, so that
# nothing which is given this folder can sign a certificate of its own.
openssl req -x509 -newkey ec -pkeyopt ec_paramgen_curve:P-256 -nodes \
    -keyout "$DIR/ca.key" -out "$DIR/ca.crt" -days 825 \
    -subj "/CN=Anahita federation lab authority" \
    -addext "basicConstraints=critical,CA:TRUE" \
    -addext "keyUsage=critical,keyCertSign,cRLSign" >/dev/null 2>&1

names=""
for name in $NAMES; do
    names="${names:+$names,}DNS:$name"
done

cat > "$DIR/tls.ext" <<EXT
basicConstraints = CA:FALSE
keyUsage = digitalSignature
extendedKeyUsage = serverAuth
subjectAltName = $names
EXT

openssl req -newkey ec -pkeyopt ec_paramgen_curve:P-256 -nodes \
    -keyout "$DIR/tls.key" -out "$DIR/tls.csr" \
    -subj "/CN=anahita.test" >/dev/null 2>&1

openssl x509 -req -in "$DIR/tls.csr" -CA "$DIR/ca.crt" -CAkey "$DIR/ca.key" \
    -CAcreateserial -out "$DIR/tls.crt" -days 825 -extfile "$DIR/tls.ext" >/dev/null 2>&1

rm -f "$DIR/ca.key" "$DIR/ca.srl" "$DIR/tls.csr" "$DIR/tls.ext"
chmod 644 "$DIR/tls.key"

echo "fedi-lab-certs: done. To open the lab in a browser without warnings, trust $DIR/ca.crt."
