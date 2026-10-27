# Services over gRPC

[Documentation](README.md) › [Developing](developing.md) › gRPC

A service of your own, running in the same cluster as an Anahita site, can
call Anahita's three gRPC services directly: the graph of profiles, posts
and the links between them, the files, and mail. It may be written in any
language that has gRPC, and be under any licence: the definitions it is made
from are Apache 2.0 ([Licensing](licensing.md)).

Most things built on Anahita should use [the API](api.md) instead: an app, a
bot, or a service outside the cluster. The gRPC services are the inside of
a site, and trust whoever calls them (see [Checking access](#checking-access)).

## Contents

- [What you need](#what-you-need)
- [A certificate](#a-certificate)
- [Generating the code](#generating-the-code)
- [Reading a node](#reading-a-node)
- [Checking access](#checking-access)
- [Trying it on your own machine](#trying-it-on-your-own-machine)

## What you need

| What | Where |
| --- | --- |
| The definitions | `anahita-libs/proto`: 22 `.proto` files, imported as `proto/graphpb/node.proto` and the like, so code is generated from `anahita-libs` |
| A certificate from the site's own authority | [A certificate](#a-certificate) |
| The services' addresses, inside the cluster | `graph-grpc-service:50051`, `file-grpc-service:50051` and `mail-grpc-service:50051`, the `*_SERVICE_ADDR` settings in `k8s/base/config.env` |
| gRPC for your language | Below, with the versions these examples were tried with |

Every example here is in [`examples/grpc/`](../examples/grpc/), one folder
for each language, and reads the same two settings Anahita's services do:
`GRAPH_SERVICE_ADDR`, and `GRPC_TLS_DIR`, the folder holding the
certificate (`/run/secrets/grpc` unless said). They are Apache 2.0, like the
definitions, so that they can be copied.

## A certificate

Anahita's services accept only callers with a certificate from the site's
own authority, `anahita-grpc`, which cert-manager keeps in the cluster
([Certificates between the services](deploying.md#certificates-between-the-services)).
Ask it for one for your service:

```yaml
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: anahita-grpc-my-service
  namespace: anahita
spec:
  secretName: anahita-grpc-my-service
  commonName: my-service
  usages:
    - client auth
  duration: 2160h
  privateKey:
    algorithm: ECDSA
    size: 256
  issuerRef:
    name: anahita-grpc
    kind: Issuer
```

and mount its Secret where your service reads it:

```yaml
      containers:
        - name: my-service
          volumeMounts:
            - name: grpc-tls
              mountPath: /run/secrets/grpc
              readOnly: true
      volumes:
        - name: grpc-tls
          secret:
            secretName: anahita-grpc-my-service
```

The Secret holds `tls.crt`, `tls.key` and `ca.crt`. cert-manager renews it
before it runs out, in place; a service that keeps running for longer than
the certificate's 90 days must read it again, or be restarted.

## Generating the code

The code is generated from the definitions with
[buf](https://buf.build/docs/installation/), with plugins it fetches itself,
so no compiler or plugin is needed on your machine. Each folder has its own
`buf.gen.yaml`, with the plugins' versions; run, in it:

```sh
buf generate ../../../anahita-libs --path ../../../anahita-libs/proto/graphpb
```

The generated code goes into `gen/`. Node.js reads the `.proto` files when
it starts, so nothing is generated for it. The libraries each needs:

**Go**

```sh
# go.mod: google.golang.org/grpc v1.76.0, google.golang.org/protobuf v1.36.10.
# Inside a checkout of anahita-services, prefix go with GOWORK=off: its
# go.work does not list the example.
buf generate ../../../anahita-libs --path ../../../anahita-libs/proto/graphpb
go mod tidy
```

**Node.js**

```sh
# package.json: @grpc/grpc-js 1.14.6, @grpc/proto-loader 0.8.1
npm install
```

**Python**

```sh
# requirements.txt: grpcio 1.84.0, protobuf 7.36.2
pip install -r requirements.txt
buf generate ../../../anahita-libs --path ../../../anahita-libs/proto/graphpb
```

**Ruby**

```sh
# Gemfile: grpc 1.84.0, google-protobuf 4.36.2. A compiler is needed for one
# of their dependencies, as in the full ruby image, not ruby:slim.
bundle install
buf generate ../../../anahita-libs --path ../../../anahita-libs/proto/graphpb
```

**PHP**

```sh
# PHP's grpc extension, as a client; serving gRPC from PHP needs a
# long-running server such as RoadRunner.
pecl install grpc
# composer.json: grpc/grpc 1.82.0, google/protobuf 5.36.2
composer install
buf generate ../../../anahita-libs --path ../../../anahita-libs/proto/graphpb
```

## Reading a node

Each example reads one node by its id, with `NodeService.Read`, and prints
its type, alias and name:

```text
1 node.actor.person-service.person.v1 @dax Dax
```

**Go**

```go
cert, err := tls.LoadX509KeyPair(filepath.Join(tlsDir, "tls.crt"), filepath.Join(tlsDir, "tls.key"))
if err != nil {
	log.Fatal(err)
}
ca, err := os.ReadFile(filepath.Join(tlsDir, "ca.crt"))
if err != nil {
	log.Fatal(err)
}
pool := x509.NewCertPool()
pool.AppendCertsFromPEM(ca)
creds := credentials.NewTLS(&tls.Config{
	MinVersion:   tls.VersionTLS13,
	Certificates: []tls.Certificate{cert},
	RootCAs:      pool,
})

conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(creds))
if err != nil {
	log.Fatal(err)
}
defer conn.Close()

ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()
reply, err := graphpb.NewNodeServiceClient(conn).Read(ctx, &graphpb.NodeReadRequest{Id: id})
if err != nil {
	log.Fatal(err)
}

node := reply.GetNode()
fmt.Println(node.GetId(), node.GetType(), "@"+node.GetAlias(), node.GetName())
```

**Node.js**

```js
const definition = protoLoader.loadSync('proto/graphpb/node.proto', {
  includeDirs: [protoRoot],
  longs: String,
  defaults: true,
});
const { graph } = grpc.loadPackageDefinition(definition);
const read = (name) => fs.readFileSync(path.join(tlsDir, name));

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
```

**Python**

```python
credentials = grpc.ssl_channel_credentials(
    root_certificates=read("ca.crt"),
    private_key=read("tls.key"),
    certificate_chain=read("tls.crt"),
)

with grpc.secure_channel(address, credentials) as channel:
    nodes = node_pb2_grpc.NodeServiceStub(channel)
    reply = nodes.Read(node_pb2.NodeReadRequest(id=node_id), timeout=5)

node = reply.node
print(node.id, node.type, "@" + node.alias, node.name)
```

**Ruby**

```ruby
credentials = GRPC::Core::ChannelCredentials.new(read.('ca.crt'), read.('tls.key'), read.('tls.crt'))

nodes = Graph::NodeService::Stub.new(address, credentials)
reply = nodes.read(Graph::NodeReadRequest.new(id: node_id), deadline: Time.now + 5)

node = reply.node
puts [node.id, node.type, "@#{node.alias}", node.name].join(' ')
```

**PHP**

```php
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
```

The whole of each, with where it reads its settings and the node's id, is
in its folder in `examples/grpc/`. The other calls are in the `.proto`
files: each `service` block lists what it answers, and each message what it
takes and gives.

## Checking access

**The gRPC services do not check who may see what.** A node read from the
graph comes back whoever it belongs to and whoever may see it: a post only
its author's followers may read, a profile that is private, a person who has
blocked another. Anahita's own services check before they answer anybody,
with the rules in `anahita-libs/permissions`
([Permissions](permissions.md)). A service of yours that shows what it reads
to people must make the same checks itself, or ask through [the API](api.md),
which makes them for it, as the person who is asking.

Any holder of a certificate from the authority may call any of the three
services, so keep the Secret as you would a password.

## Trying it on your own machine

On the [k3d cluster](deploying.md#with-k3d): ask for the certificate above,
copy its three files out of the Secret, forward graph-grpc's port, and run
an example in a container that knows the service by its name in the
cluster, so that its certificate is accepted:

```sh
kubectl --kubeconfig ~/.config/k3d/kubeconfig-anahita.yaml -n anahita \
  port-forward --address 127.0.0.1 svc/graph-grpc-service 50051:50051

docker run --rm --add-host graph-grpc-service:host-gateway \
  -v "$PWD/certs:/run/secrets/grpc:ro" -v "$PWD:/example" -w /example \
  python:3.12-slim sh -c 'pip install -r requirements.txt && python client.py 1'
```

The examples were each tried this way, against a site's graph, on
2026-10-26. Where `host-gateway` gives an IPv6 address that does not answer,
as it can for PHP's gRPC, give the host's IPv4 address instead
(`getent ahostsv4 host.docker.internal` in a container).
