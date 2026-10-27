// Reads one node from Anahita's graph service over gRPC, with mutual TLS.
// The .proto files are read when it starts, so nothing is generated.
//
//   npm install
//   node index.js 42
const fs = require('fs');
const path = require('path');
const grpc = require('@grpc/grpc-js');
const protoLoader = require('@grpc/proto-loader');

const id = process.argv[2];
const tlsDir = process.env.GRPC_TLS_DIR || '/run/secrets/grpc';
const address = process.env.GRAPH_SERVICE_ADDR || 'graph-grpc-service:50051';
const protoRoot = process.env.ANAHITA_PROTO_ROOT || path.join(__dirname, '../../../anahita-libs');

const definition = protoLoader.loadSync('proto/graphpb/node.proto', {
  includeDirs: [protoRoot],
  longs: String,
  defaults: true,
});
const { graph } = grpc.loadPackageDefinition(definition);
const read = (name) => fs.readFileSync(path.join(tlsDir, name));

// This service's certificate, and the cluster's authority, which signed it
// and every Anahita service's.
const credentials = grpc.credentials.createSsl(read('ca.crt'), read('tls.key'), read('tls.crt'));

const nodes = new graph.NodeService(address, credentials);
nodes.Read({ id }, { deadline: Date.now() + 5000 }, (error, reply) => {
  nodes.close();
  if (error) {
    console.error(error.message);
    process.exit(1);
  }
  const { node } = reply;
  console.log(node.id, node.type, `@${node.alias}`, node.name);
});
