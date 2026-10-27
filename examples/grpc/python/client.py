"""Reads one node from Anahita's graph service over gRPC, with mutual TLS.

    pip install -r requirements.txt
    buf generate ../../../anahita-libs --path ../../../anahita-libs/proto/graphpb
    python client.py 42
"""
import os
import sys

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "gen"))

import grpc  # noqa: E402
from proto.graphpb import node_pb2, node_pb2_grpc  # noqa: E402

node_id = int(sys.argv[1])
tls_dir = os.environ.get("GRPC_TLS_DIR", "/run/secrets/grpc")
address = os.environ.get("GRAPH_SERVICE_ADDR", "graph-grpc-service:50051")


def read(name):
    with open(os.path.join(tls_dir, name), "rb") as f:
        return f.read()


# This service's certificate, and the cluster's authority, which signed it
# and every Anahita service's.
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
