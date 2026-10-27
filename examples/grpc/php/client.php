<?php
// Reads one node from Anahita's graph service over gRPC, with mutual TLS.
// Needs PHP's grpc extension (pecl install grpc).
//
//   composer install
//   buf generate ../../../anahita-libs --path ../../../anahita-libs/proto/graphpb
//   php client.php 42

require __DIR__ . '/vendor/autoload.php';

$nodeId = (int) $argv[1];
$tlsDir = getenv('GRPC_TLS_DIR') ?: '/run/secrets/grpc';
$address = getenv('GRAPH_SERVICE_ADDR') ?: 'graph-grpc-service:50051';
$read = fn (string $name): string => file_get_contents("$tlsDir/$name");

// This service's certificate, and the cluster's authority, which signed it
// and every Anahita service's.
$nodes = new \Graph\NodeServiceClient($address, [
    'credentials' => \Grpc\ChannelCredentials::createSsl($read('ca.crt'), $read('tls.key'), $read('tls.crt')),
]);

[$reply, $status] = $nodes->Read(
    new \Graph\NodeReadRequest(['id' => $nodeId]),
    [],
    ['timeout' => 5_000_000],
)->wait();
if ($status->code !== \Grpc\STATUS_OK) {
    fwrite(STDERR, $status->details . "\n");
    exit(1);
}

$node = $reply->getNode();
echo $node->getId(), ' ', $node->getType(), ' @', $node->getAlias(), ' ', $node->getName(), "\n";
