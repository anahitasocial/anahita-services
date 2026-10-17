gen-proto:
	protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative proto/filepb/file.proto
	protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative proto/mailpb/mail.proto
	protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative \
		proto/graphpb/context.proto \
		proto/graphpb/node.proto \
		proto/graphpb/edge.proto \
		proto/graphpb/graph.proto \
		proto/graphpb/socialgraph_block.proto \
		proto/graphpb/socialgraph_follow.proto \
		proto/graphpb/socialgraph.proto \
		proto/graphpb/subscription_rel.proto \
		proto/graphpb/like_rel.proto \
		proto/graphpb/actor.proto \
		proto/graphpb/actor_admin.proto \
		proto/graphpb/event.proto \
		proto/graphpb/medium.proto \
		proto/graphpb/feed_leaders.proto \
		proto/graphpb/feed_actor.proto \
		proto/graphpb/feed_custom.proto \
		proto/graphpb/repost.proto \
		proto/graphpb/search.proto \
		proto/graphpb/hashtag.proto
	protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative proto/typespb/types.proto

