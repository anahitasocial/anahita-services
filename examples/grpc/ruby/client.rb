# Reads one node from Anahita's graph service over gRPC, with mutual TLS.
#
#   bundle install
#   buf generate ../../../anahita-libs --path ../../../anahita-libs/proto/graphpb
#   bundle exec ruby client.rb 42
$LOAD_PATH.unshift(File.join(__dir__, 'gen'))

require 'grpc'
require 'proto/graphpb/node_services_pb'

node_id = Integer(ARGV[0])
tls_dir = ENV.fetch('GRPC_TLS_DIR', '/run/secrets/grpc')
address = ENV.fetch('GRAPH_SERVICE_ADDR', 'graph-grpc-service:50051')
read = ->(name) { File.read(File.join(tls_dir, name)) }

# This service's certificate, and the cluster's authority, which signed it
# and every Anahita service's.
credentials = GRPC::Core::ChannelCredentials.new(read.('ca.crt'), read.('tls.key'), read.('tls.crt'))

nodes = Graph::NodeService::Stub.new(address, credentials)
reply = nodes.read(Graph::NodeReadRequest.new(id: node_id), deadline: Time.now + 5)

node = reply.node
puts [node.id, node.type, "@#{node.alias}", node.name].join(' ')
