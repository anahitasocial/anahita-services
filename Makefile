REPOSITORY=anahita
DOCKERFILES=dockerfiles

ABUSE_REPORT_BINARY=app-binary
ABUSE_REPORT_VERSION=latest
ABUSE_REPORT_SRV=abuse-report-service

AUTH_BINARY=app-binary
AUTH_VERSION=latest
AUTH_SRV=auth-service

COMMENT_BINARY=app-binary
COMMENT_VERSION=latest
COMMENT_SRV=comment-service

GRAPH_BINARY=app-binary
GRAPH_VERSION=latest
GRAPH_SRV=graph-grpc-service
SWEEPER_JOB=sweeper-job

GROUP_BINARY=app-binary
GROUP_VERSION=latest
GROUP_SRV=group-service

LIKE_BINARY=app-binary
LIKE_VERSION=latest
LIKE_SRV=like-service

NGINX_VERSION=latest
NGINX_SRV=nginx

PERSON_BINARY=app-binary
PERSON_VERSION=latest
PERSON_SRV=person-service

TEXT_BINARY=app-binary
TEXT_VERSION=latest
TEXT_SRV=text-service

PHOTO_BINARY=app-binary
PHOTO_VERSION=latest
PHOTO_SRV=photo-service

HASHTAG_BINARY=app-binary
HASHTAG_VERSION=latest
HASHTAG_SRV=hashtag-service

LOCATION_BINARY=app-binary
LOCATION_VERSION=latest
LOCATION_SRV=location-service

SEARCH_BINARY=app-binary
SEARCH_VERSION=latest
SEARCH_SRV=search-service

SOCIALGRAPH_BINARY=app-binary
SOCIALGRAPH_VERSION=latest
SOCIALGRAPH_SRV=socialgraph-service

STORY_BINARY=app-binary
STORY_VERSION=latest
STORY_SRV=story-service

NOTIFICATION_BINARY=app-binary
NOTIFICATION_VERSION=latest
NOTIFICATION_SRV=notification-service

FILE_BINARY=app-binary
FILE_VERSION=latest
FILE_SRV=file-grpc-service

AVATAR_BINARY=app-binary
AVATAR_VERSION=latest
AVATAR_SRV=avatar-service

COVER_BINARY=app-binary
COVER_VERSION=latest
COVER_SRV=cover-service

MAIL_BINARY=app-binary
MAIL_VERSION=latest
MAIL_SRV=mail-grpc-service

MIGRATION_VERSION=latest
MIGRATION_JOB=migration-job

FEED_BINARY=app-binary
FEED_VERSION=latest
FEED_SRV=feed-service

GOOS = linux
GOARCH = $(shell go env GOARCH)

## check_docker: checks if Docker daemon is running
check_docker:
	@docker info > /dev/null 2>&1 || { echo "Docker daemon is not running. Please start Docker and try again."; exit 1; }

## up: starts all containers in the background without forcing build
up:
	@echo "Starting docker images..."
	@./scripts/grpc-certs-init.sh
	docker-compose up -d
	@./scripts/storage-init.sh
	@echo "Docker images started!"

## storage_init: prepares the local Garage store (role, bucket, access key); safe to repeat
storage_init:
	@./scripts/storage-init.sh

## down: stop docker compose
down:
	@echo "Stopping docker images..."
	docker-compose down
	@echo "Docker stopped!"

## build_dockerfiles: builds all dockerfile images
build_dockerfiles: check_docker build_abuse_report build_auth build_comment build_graph build_sweeper build_group build_like build_person build_text build_photo build_hashtag build_location build_search build_socialgraph build_story build_notification build_file build_avatar build_cover build_mail build_feed
	@echo "Building dockerfiles..."
	docker build -f ${DOCKERFILES}/${ABUSE_REPORT_SRV}.dockerfile -t ${REPOSITORY}/${ABUSE_REPORT_SRV}:${ABUSE_REPORT_VERSION} .
	docker build -f ${DOCKERFILES}/${AUTH_SRV}.dockerfile -t ${REPOSITORY}/${AUTH_SRV}:${AUTH_VERSION} .
	docker build -f ${DOCKERFILES}/${COMMENT_SRV}.dockerfile -t ${REPOSITORY}/${COMMENT_SRV}:${COMMENT_VERSION} .
	docker build -f ${DOCKERFILES}/${GRAPH_SRV}.dockerfile -t ${REPOSITORY}/${GRAPH_SRV}:${GRAPH_VERSION} .
	docker build -f ${DOCKERFILES}/${SWEEPER_JOB}.dockerfile -t ${REPOSITORY}/${SWEEPER_JOB}:${GRAPH_VERSION} .
	docker build -f ${DOCKERFILES}/${GROUP_SRV}.dockerfile -t ${REPOSITORY}/${GROUP_SRV}:${GROUP_VERSION} .
	docker build -f ${DOCKERFILES}/${LIKE_SRV}.dockerfile -t ${REPOSITORY}/${LIKE_SRV}:${LIKE_VERSION} .
	docker build -f ${DOCKERFILES}/${PERSON_SRV}.dockerfile -t ${REPOSITORY}/${PERSON_SRV}:${PERSON_VERSION} .
	docker build -f ${DOCKERFILES}/${TEXT_SRV}.dockerfile -t ${REPOSITORY}/${TEXT_SRV}:${TEXT_VERSION} .
	docker build -f ${DOCKERFILES}/${PHOTO_SRV}.dockerfile -t ${REPOSITORY}/${PHOTO_SRV}:${PHOTO_VERSION} .
	docker build -f ${DOCKERFILES}/${HASHTAG_SRV}.dockerfile -t ${REPOSITORY}/${HASHTAG_SRV}:${HASHTAG_VERSION} .
	docker build -f ${DOCKERFILES}/${LOCATION_SRV}.dockerfile -t ${REPOSITORY}/${LOCATION_SRV}:${LOCATION_VERSION} .
	docker build -f ${DOCKERFILES}/${NGINX_SRV}.dockerfile -t ${REPOSITORY}/${NGINX_SRV}:${NGINX_VERSION} .
	docker build -f ${DOCKERFILES}/${SEARCH_SRV}.dockerfile -t ${REPOSITORY}/${SEARCH_SRV}:${SEARCH_VERSION} .
	docker build -f ${DOCKERFILES}/${SOCIALGRAPH_SRV}.dockerfile -t ${REPOSITORY}/${SOCIALGRAPH_SRV}:${SOCIALGRAPH_VERSION} .
	docker build -f ${DOCKERFILES}/${STORY_SRV}.dockerfile -t ${REPOSITORY}/${STORY_SRV}:${STORY_VERSION} .
	docker build -f ${DOCKERFILES}/${NOTIFICATION_SRV}.dockerfile -t ${REPOSITORY}/${NOTIFICATION_SRV}:${NOTIFICATION_VERSION} .
	docker build -f ${DOCKERFILES}/${FILE_SRV}.dockerfile -t ${REPOSITORY}/${FILE_SRV}:${FILE_VERSION} .
	docker build -f ${DOCKERFILES}/${AVATAR_SRV}.dockerfile -t ${REPOSITORY}/${AVATAR_SRV}:${AVATAR_VERSION} .
	docker build -f ${DOCKERFILES}/${COVER_SRV}.dockerfile -t ${REPOSITORY}/${COVER_SRV}:${COVER_VERSION} .
	docker build -f ${DOCKERFILES}/${MAIL_SRV}.dockerfile -t ${REPOSITORY}/${MAIL_SRV}:${MAIL_VERSION} .
	docker build -f ${DOCKERFILES}/${MIGRATION_JOB}.dockerfile -t ${REPOSITORY}/${MIGRATION_JOB}:${MIGRATION_VERSION} .
	docker build -f ${DOCKERFILES}/${FEED_SRV}.dockerfile -t ${REPOSITORY}/${FEED_SRV}:${FEED_VERSION} .
	@echo "Done!"

## push_dockerfiles: pushes tagged versions to docker hub
push_dockerfiles: check_docker build_dockerfiles
	docker push ${REPOSITORY}/${ABUSE_REPORT_SRV}:${ABUSE_REPORT_VERSION}
	docker push ${REPOSITORY}/${AUTH_SRV}:${AUTH_VERSION}
	docker push ${REPOSITORY}/${COMMENT_SRV}:${COMMENT_VERSION}
	docker push ${REPOSITORY}/${GRAPH_SRV}:${GRAPH_VERSION}
	docker push ${REPOSITORY}/${SWEEPER_JOB}:${GRAPH_VERSION}
	docker push ${REPOSITORY}/${GROUP_SRV}:${GROUP_VERSION}
	docker push ${REPOSITORY}/${LIKE_SRV}:${LIKE_VERSION}
	docker push ${REPOSITORY}/${PERSON_SRV}:${PERSON_VERSION}
	docker push ${REPOSITORY}/${TEXT_SRV}:${TEXT_VERSION}
	docker push ${REPOSITORY}/${PHOTO_SRV}:${PHOTO_VERSION}
	docker push ${REPOSITORY}/${HASHTAG_SRV}:${HASHTAG_VERSION}
	docker push ${REPOSITORY}/${LOCATION_SRV}:${LOCATION_VERSION}
	docker push ${REPOSITORY}/${NGINX_SRV}:${NGINX_VERSION}
	docker push ${REPOSITORY}/${SEARCH_SRV}:${SEARCH_VERSION}
	docker push ${REPOSITORY}/${SOCIALGRAPH_SRV}:${SOCIALGRAPH_VERSION}
	docker push ${REPOSITORY}/${STORY_SRV}:${STORY_VERSION}
	docker push ${REPOSITORY}/${NOTIFICATION_SRV}:${NOTIFICATION_VERSION}
	docker push ${REPOSITORY}/${FILE_SRV}:${FILE_VERSION}
	docker push ${REPOSITORY}/${AVATAR_SRV}:${AVATAR_VERSION}
	docker push ${REPOSITORY}/${COVER_SRV}:${COVER_VERSION}
	docker push ${REPOSITORY}/${MAIL_SRV}:${MAIL_VERSION}
	docker push ${REPOSITORY}/${MIGRATION_JOB}:${MIGRATION_VERSION}
	docker push ${REPOSITORY}/${FEED_SRV}:${FEED_VERSION}
	@echo "Done!"

## swarm_up: starts the swarm
swarm_up:
	@echo "Starting swarm..."
	docker stack deploy -c swarm.yml myapp

## swarm_down: stops the swarm
swarm_down:
	@echo "Stopping swarm..."
	docker stack rm myapp

## build_auth: builds the auth binary as a linux executable
build_auth:
	@echo "Building auth binary.."
	cd ${AUTH_SRV} && env GOOS=$(GOOS) GOARCH=$(GOARCH) CGO_ENABLED=0 go build -o ${AUTH_BINARY} ./cmd/api
	@echo "Authentication binary built!"

## build_comment: builds the comment binary as a linux executable
build_comment:
	@echo "Building comment binary.."
	cd ${COMMENT_SRV} && env GOOS=$(GOOS) GOARCH=$(GOARCH) CGO_ENABLED=0 go build -o ${COMMENT_BINARY} ./cmd/api
	@echo "Comment binary built!"

## build_graph: builds the graph binary as a linux executable
build_graph:
	@echo "Building graph binary.."
	cd ${GRAPH_SRV} && env GOOS=$(GOOS) GOARCH=$(GOARCH) CGO_ENABLED=0 go build -o ${GRAPH_BINARY} ./cmd/server
	@echo "Graph binary built!"

## build_sweeper: builds the account sweeper binary as a linux executable
build_sweeper:
	@echo "Building sweeper binary.."
	cd ${GRAPH_SRV} && env GOOS=$(GOOS) GOARCH=$(GOARCH) CGO_ENABLED=0 go build -o sweeper-binary ./cmd/sweeper
	@echo "Sweeper binary built!"

## build_group: builds the group binary as a linux executable
build_group:
	@echo "Building group binary.."
	cd ${GROUP_SRV} && env GOOS=$(GOOS) GOARCH=$(GOARCH) CGO_ENABLED=0 go build -o ${GROUP_BINARY} ./cmd/api
	@echo "Groups binary built!"

## build_like: builds the like binary as a linux executable
build_like:
	@echo "Building like binary.."
	cd ${LIKE_SRV} && env GOOS=$(GOOS) GOARCH=$(GOARCH) CGO_ENABLED=0 go build -o ${LIKE_BINARY} ./cmd/api
	@echo "Likes binary built!"

## build_person: builds the person binary as a linux executable
build_person:
	@echo "Building person binary.."
	cd ${PERSON_SRV} && env GOOS=$(GOOS) GOARCH=$(GOARCH) CGO_ENABLED=0 go build -o ${PERSON_BINARY} ./cmd/api
	@echo "People binary built!"

## build_text: builds the text binary as a linux executable
build_text:
	@echo "Building text binary.."
	cd ${TEXT_SRV} && env GOOS=$(GOOS) GOARCH=$(GOARCH) CGO_ENABLED=0 go build -o ${TEXT_BINARY} ./cmd/api
	@echo "Text binary built!"

## build_photo: builds the photo binary as a linux executable
build_photo:
	@echo "Building photo binary.."
	cd ${PHOTO_SRV} && env GOOS=$(GOOS) GOARCH=$(GOARCH) CGO_ENABLED=0 go build -o ${PHOTO_BINARY} ./cmd/api
	@echo "Photos binary built!"

## build_abuse_report: builds the abuse report binary as a linux executable
build_abuse_report:
	@echo "Building abuse report binary.."
	cd ${ABUSE_REPORT_SRV} && env GOOS=$(GOOS) GOARCH=$(GOARCH) CGO_ENABLED=0 go build -o ${ABUSE_REPORT_BINARY} ./cmd/api
	@echo "Abuse report binary built!"

## build_hashtag: builds the hashtag binary as a linux executable
build_hashtag:
	@echo "Building hashtag binary.."
	cd ${HASHTAG_SRV} && env GOOS=$(GOOS) GOARCH=$(GOARCH) CGO_ENABLED=0 go build -o ${HASHTAG_BINARY} ./cmd/api
	@echo "Hashtags binary built!"

## build_location: builds the location binary as a linux executable
build_location:
	@echo "Building location binary.."
	cd ${LOCATION_SRV} && env GOOS=$(GOOS) GOARCH=$(GOARCH) CGO_ENABLED=0 go build -o ${LOCATION_BINARY} ./cmd/api
	@echo "Locations binary built!"

## build_search: builds the search binary as a linux executable
build_search:
	@echo "Building search binary.."
	cd ${SEARCH_SRV} && env GOOS=$(GOOS) GOARCH=$(GOARCH) CGO_ENABLED=0 go build -o ${SEARCH_BINARY} ./cmd/api
	@echo "Search binary built!"

## build_socialgraph: builds the socialgraph binary as a linux executable
build_socialgraph:
	@echo "Building socialgraph binary.."
	cd ${SOCIALGRAPH_SRV} && env GOOS=$(GOOS) GOARCH=$(GOARCH) CGO_ENABLED=0 go build -o ${SOCIALGRAPH_BINARY} ./cmd/api
	@echo "Socialgraph binary built!"

## build_story: builds the story binary as a linux executable
build_story:
	@echo "Building story binary.."
	cd ${STORY_SRV} && env GOOS=$(GOOS) GOARCH=$(GOARCH) CGO_ENABLED=0 go build -o ${STORY_BINARY} ./cmd/api
	@echo "Stories binary built!"

## build_notification: builds the notification binary as a linux executable
build_notification:
	@echo "Building notification binary.."
	cd ${NOTIFICATION_SRV} && env GOOS=$(GOOS) GOARCH=$(GOARCH) CGO_ENABLED=0 go build -o ${NOTIFICATION_BINARY} ./cmd/api
	@echo "Notifications binary built!"

## build_file: builds the file binary as a linux executable
build_file:
	@echo "Building file binary.."
	cd ${FILE_SRV} && env GOOS=$(GOOS) GOARCH=$(GOARCH) CGO_ENABLED=0 go build -o ${FILE_BINARY} ./cmd/server
	@echo "File binary built!"

## build_avatar: builds the avatar binary as a linux executable
build_avatar:
	@echo "Building avatar binary.."
	cd ${AVATAR_SRV} && env GOOS=$(GOOS) GOARCH=$(GOARCH) CGO_ENABLED=0 go build -o ${AVATAR_BINARY} ./cmd/api
	@echo "Avatar binary built!"

## build_cover: builds the cover binary as a linux executable
build_cover:
	@echo "Building cover binary.."
	cd ${COVER_SRV} && env GOOS=$(GOOS) GOARCH=$(GOARCH) CGO_ENABLED=0 go build -o ${COVER_BINARY} ./cmd/api
	@echo "Cover binary built!"

## build_mail: builds the mail binary as a linux executable
build_mail:
	@echo "Building mail binary.."
	cd ${MAIL_SRV} && env GOOS=$(GOOS) GOARCH=$(GOARCH) CGO_ENABLED=0 go build -o ${MAIL_BINARY} ./cmd/server
	@echo "Mail binary built!"

## build_feed: builds the feed binary as a linux executable
build_feed:
	@echo "Building feed binary.."
	cd ${FEED_SRV} && env GOOS=$(GOOS) GOARCH=$(GOARCH) CGO_ENABLED=0 go build -o ${FEED_BINARY} ./cmd/api
	@echo "Feed binary built!"

## up_build: stops docker-compose (if running), builds all projects and starts docker compose
# up_build: build_auth build_person build_broker build_listener build_logger build_mail
up_build: build_abuse_report build_auth build_comment build_graph build_group build_like build_person build_text build_photo build_hashtag build_location build_search build_socialgraph build_story build_notification build_file build_avatar build_cover build_mail build_feed
	@echo "Stopping docker images (if running...)"
	docker-compose down
	@echo "Building (when required) and starting docker images..."
	@./scripts/grpc-certs-init.sh
	docker-compose up --build -d
	@./scripts/storage-init.sh
	@echo "Docker images built and started!"

## auth: stops auth-service, removes docker image, builds service, and starts it
auth: build_auth
	@echo "Building ${AUTH_SRV} docker image..."
	- docker-compose stop ${AUTH_SRV}
	- docker-compose rm -f ${AUTH_SRV}
	docker-compose up --build -d ${AUTH_SRV}
	docker-compose start ${AUTH_SRV}
	@echo "${AUTH_SRV} built and started!"

## comment: stops comment-service, removes docker image, builds service, and starts it
comment: build_comment
	@echo "Building ${COMMENT_SRV} docker image..."
	- docker-compose stop ${COMMENT_SRV}
	- docker-compose rm -f ${COMMENT_SRV}
	docker-compose up --build -d ${COMMENT_SRV}
	docker-compose start ${COMMENT_SRV}
	@echo "${COMMENT_SRV} built and started!"

## graph: stops graph-grpc-service, removes docker image, builds service, and starts it
graph: build_graph
	@echo "Building ${GRAPH_SRV} docker image..."
	- docker-compose stop ${GRAPH_SRV}
	- docker-compose rm -f ${GRAPH_SRV}
	docker-compose up --build -d ${GRAPH_SRV}
	docker-compose start ${GRAPH_SRV}
	@echo "${GRAPH_SRV} built and started!"

## group: stops group-service, removes docker image, builds service, and starts it
group: build_group
	@echo "Building ${GROUP_SRV} docker image..."
	- docker-compose stop ${GROUP_SRV}
	- docker-compose rm -f ${GROUP_SRV}
	docker-compose up --build -d ${GROUP_SRV}
	docker-compose start ${GROUP_SRV}
	@echo "${GROUP_SRV} built and started!"

## like: stops like-service, removes docker image, builds service, and starts it
like: build_like
	@echo "Building ${LIKE_SRV} docker image..."
	- docker-compose stop ${LIKE_SRV}
	- docker-compose rm -f ${LIKE_SRV}
	docker-compose up --build -d ${LIKE_SRV}
	docker-compose start ${LIKE_SRV}
	@echo "${LIKE_SRV} built and started!"

## person: stops person-service, removes docker image, builds service, and starts it
person: build_person
	@echo "Building ${PERSON_SRV} docker image..."
	- docker-compose stop ${PERSON_SRV}
	- docker-compose rm -f ${PERSON_SRV}
	docker-compose up --build -d ${PERSON_SRV}
	docker-compose start ${PERSON_SRV}
	@echo "${PERSON_SRV} built and started!"

## text: stops text-service, removes docker image, builds service, and starts it
text: build_text
	@echo "Building ${TEXT_SRV} docker image..."
	- docker-compose stop ${TEXT_SRV}
	- docker-compose rm -f ${TEXT_SRV}
	docker-compose up --build -d ${TEXT_SRV}
	docker-compose start ${TEXT_SRV}
	@echo "${TEXT_SRV} built and started!"

## photo: stops photo-service, removes docker image, builds service, and starts it
photo: build_photo
	@echo "Building ${PHOTO_SRV} docker image..."
	- docker-compose stop ${PHOTO_SRV}
	- docker-compose rm -f ${PHOTO_SRV}
	docker-compose up --build -d ${PHOTO_SRV}
	docker-compose start ${PHOTO_SRV}
	@echo "${PHOTO_SRV} built and started!"

## abuse_report: stops abuse-report-service, removes docker image, builds service, and starts it
abuse_report: build_abuse_report
	@echo "Building ${ABUSE_REPORT_SRV} docker image..."
	- docker-compose stop ${ABUSE_REPORT_SRV}
	- docker-compose rm -f ${ABUSE_REPORT_SRV}
	docker-compose up --build -d ${ABUSE_REPORT_SRV}
	docker-compose start ${ABUSE_REPORT_SRV}
	@echo "${ABUSE_REPORT_SRV} built and started!"

## hashtag: stops hashtag-service, removes docker image, builds service, and starts it
hashtag: build_hashtag
	@echo "Building ${HASHTAG_SRV} docker image..."
	- docker-compose stop ${HASHTAG_SRV}
	- docker-compose rm -f ${HASHTAG_SRV}
	docker-compose up --build -d ${HASHTAG_SRV}
	docker-compose start ${HASHTAG_SRV}
	@echo "${HASHTAG_SRV} built and started!"

## location: stops location-service, removes docker image, builds service, and starts it
location: build_location
	@echo "Building ${LOCATION_SRV} docker image..."
	- docker-compose stop ${LOCATION_SRV}
	- docker-compose rm -f ${LOCATION_SRV}
	docker-compose up --build -d ${LOCATION_SRV}
	docker-compose start ${LOCATION_SRV}
	@echo "${LOCATION_SRV} built and started!"

## nginx: stops nginx service, removes docker image, builds service, and starts it
# Phony because a directory has the same name, which make would otherwise
# take for an up-to-date target and do nothing.
.PHONY: nginx
nginx:
	@echo "Building ${NGINX_SRV} docker image..."
	- docker-compose stop ${NGINX_SRV}
	- docker-compose rm -f ${NGINX_SRV}
	docker-compose up --build -d ${NGINX_SRV}
	docker-compose start ${NGINX_SRV}
	@echo "${NGINX_SRV} built and started!"

## search: stops search-service, removes docker image, builds service, and starts it
search: build_search
	@echo "Building ${SEARCH_SRV} docker image..."
	- docker-compose stop ${SEARCH_SRV}
	- docker-compose rm -f ${SEARCH_SRV}
	docker-compose up --build -d ${SEARCH_SRV}
	docker-compose start ${SEARCH_SRV}
	@echo "${SEARCH_SRV} built and started!"

## socialgraph: stops socialgraph-service, removes docker image, builds service, and starts it
socialgraph: build_socialgraph
	@echo "Building ${SOCIALGRAPH_SRV} docker image..."
	- docker-compose stop ${SOCIALGRAPH_SRV}
	- docker-compose rm -f ${SOCIALGRAPH_SRV}
	docker-compose up --build -d ${SOCIALGRAPH_SRV}
	docker-compose start ${SOCIALGRAPH_SRV}
	@echo "${SOCIALGRAPH_SRV} built and started!"

## story: stops story-service, removes docker image, builds service, and starts it
story: build_story
	@echo "Building ${STORY_SRV} docker image..."
	- docker-compose stop ${STORY_SRV}
	- docker-compose rm -f ${STORY_SRV}
	docker-compose up --build -d ${STORY_SRV}
	docker-compose start ${STORY_SRV}
	@echo "${STORY_SRV} built and started!"

## notification: stops notification-service, removes docker image, builds service, and starts it
notification: build_notification
	@echo "Building ${NOTIFICATION_SRV} docker image..."
	- docker-compose stop ${NOTIFICATION_SRV}
	- docker-compose rm -f ${NOTIFICATION_SRV}
	docker-compose up --build -d ${NOTIFICATION_SRV}
	docker-compose start ${NOTIFICATION_SRV}
	@echo "${NOTIFICATION_SRV} built and started!"

## file: stops file-grpc-service, removes docker image, builds service, and starts it
file: build_file
	@echo "Building ${FILE_SRV} docker image..."
	- docker-compose stop ${FILE_SRV}
	- docker-compose rm -f ${FILE_SRV}
	docker-compose up --build -d ${FILE_SRV}
	docker-compose start ${FILE_SRV}
	@echo "${FILE_SRV} built and started!"

## avatar: stops avatar-service, removes docker image, builds service, and starts it
avatar: build_avatar
	@echo "Building ${AVATAR_SRV} docker image..."
	- docker-compose stop ${AVATAR_SRV}
	- docker-compose rm -f ${AVATAR_SRV}
	docker-compose up --build -d ${AVATAR_SRV}
	docker-compose start ${AVATAR_SRV}
	@echo "${AVATAR_SRV} built and started!"

## cover: stops cover-service, removes docker image, builds service, and starts it
cover: build_cover
	@echo "Building ${COVER_SRV} docker image..."
	- docker-compose stop ${COVER_SRV}
	- docker-compose rm -f ${COVER_SRV}
	docker-compose up --build -d ${COVER_SRV}
	docker-compose start ${COVER_SRV}
	@echo "${COVER_SRV} built and started!"

## mail: stops mail-service, removes docker image, builds service, and starts it
mail: build_mail
	@echo "Building ${MAIL_SRV} docker image..."
	- docker-compose stop ${MAIL_SRV}
	- docker-compose rm -f ${MAIL_SRV}
	docker-compose up --build -d ${MAIL_SRV}
	docker-compose start ${MAIL_SRV}
	@echo "${MAIL_SRV} built and started!"

## migration: runs the migration job
migration:
	@echo "Running migration job..."
	- docker-compose stop ${MIGRATION_JOB}
	- docker-compose rm -f ${MIGRATION_JOB}
	docker-compose up --build -d ${MIGRATION_JOB}
	docker-compose start ${MIGRATION_JOB}
	@echo "Migration job ran!"

## feed: builds the feed service
feed: build_feed
	@echo "Building ${FEED_SRV} docker image..."
	- docker-compose stop ${FEED_SRV}
	- docker-compose rm -f ${FEED_SRV}
	docker-compose up --build -d ${FEED_SRV}
	docker-compose start ${FEED_SRV}
	@echo "${FEED_SRV} built and started!"

## clean: runs go clean and deletes binaries
clean:
	@echo "Cleaning..."
	@cd ${AUTH_SRV} && rm -f ${AUTH_BINARY}
	@cd ${AUTH_SRV} && go clean
	@cd ${COMMENT_SRV} && rm -f ${COMMENT_BINARY}
	@cd ${COMMENT_SRV} && go clean
	@cd ${GRAPH_SRV} && rm -f ${GRAPH_BINARY}
	@cd ${GRAPH_SRV} && go clean
	@cd ${GROUP_SRV} && rm -f ${GROUP_BINARY}
	@cd ${GROUP_SRV} && go clean
	@cd ${LIKE_SRV} && rm -f ${LIKE_BINARY}
	@cd ${LIKE_SRV} && go clean
	@cd ${PERSON_SRV} && rm -f ${PERSON_BINARY}
	@cd ${PERSON_SRV} && go clean
	@cd ${TEXT_SRV} && rm -f ${TEXT_BINARY}
	@cd ${TEXT_SRV} && go clean
	@cd ${PHOTO_SRV} && rm -f ${PHOTO_BINARY}
	@cd ${PHOTO_SRV} && go clean
	@cd ${ABUSE_REPORT_SRV} && rm -f ${ABUSE_REPORT_BINARY}
	@cd ${ABUSE_REPORT_SRV} && go clean
	@cd ${HASHTAG_SRV} && rm -f ${HASHTAG_BINARY}
	@cd ${HASHTAG_SRV} && go clean
	@cd ${LOCATION_SRV} && rm -f ${LOCATION_BINARY}
	@cd ${LOCATION_SRV} && go clean
	@cd ${SEARCH_SRV} && rm -f ${SEARCH_BINARY}
	@cd ${SEARCH_SRV} && go clean
	@cd ${SOCIALGRAPH_SRV} && rm -f ${SOCIALGRAPH_BINARY}
	@cd ${SOCIALGRAPH_SRV} && go clean
	@cd ${STORY_SRV} && rm -f ${STORY_BINARY}
	@cd ${STORY_SRV} && go clean
	@cd ${NOTIFICATION_SRV} && rm -f ${NOTIFICATION_BINARY}
	@cd ${NOTIFICATION_SRV} && go clean
	@cd ${FILE_SRV} && rm -f ${FILE_BINARY}
	@cd ${FILE_SRV} && go clean
	@cd ${AVATAR_SRV} && rm -f ${AVATAR_BINARY}
	@cd ${AVATAR_SRV} && go clean
	@cd ${COVER_SRV} && rm -f ${COVER_BINARY}
	@cd ${COVER_SRV} && go clean
	@cd ${MAIL_SRV} && rm -f ${MAIL_BINARY}
	@cd ${MAIL_SRV} && go clean
	@cd ${FEED_SRV} && rm -f ${FEED_BINARY}
	@cd ${FEED_SRV} && go clean
	@echo "Cleaned!"

## help: displays help
help: Makefile
	@echo " Choose a command:"
	@sed -n 's/^##//p' $< | column -t -s ':' |  sed -e 's/^/ /'

#
#	A local Kubernetes cluster, for trying the manifests in k8s/. Live sites
#	use k8s/overlays/generic or k8s/overlays/aws: see docs/deploying.md.
#	Day-to-day development is quicker with docker-compose (make up_build).
#

K8S_LOCAL=k8s/overlays/local
# Always the Minikube cluster, whatever kubectl currently points at, so these
# targets can never touch a real one.
KUBECTL=kubectl --context minikube
K8S_IMAGES=${ABUSE_REPORT_SRV} ${AUTH_SRV} ${AVATAR_SRV} ${COMMENT_SRV} ${COVER_SRV} ${FEED_SRV} ${FILE_SRV} ${GRAPH_SRV} ${GROUP_SRV} ${HASHTAG_SRV} ${LIKE_SRV} ${LOCATION_SRV} ${MAIL_SRV} ${MIGRATION_JOB} ${NGINX_SRV} ${NOTIFICATION_SRV} ${PERSON_SRV} ${PHOTO_SRV} ${SEARCH_SRV} ${SOCIALGRAPH_SRV} ${STORY_SRV} ${SWEEPER_JOB} ${TEXT_SRV}

## k8s-start: starts a local Minikube cluster with the ingress controller and cert-manager
k8s-start:
	@echo "Starting kubernetes..."
	minikube start --cpus 4 --memory 5g
	minikube addons enable ingress
	${KUBECTL} apply -f https://github.com/cert-manager/cert-manager/releases/latest/download/cert-manager.yaml
	${KUBECTL} -n cert-manager wait --for=condition=Available deployment --all --timeout=180s
	@echo "Done!"

## k8s-build: builds every image and loads it into Minikube
k8s-build: build_dockerfiles
	@echo "Loading images into Minikube..."
	@for image in ${K8S_IMAGES}; do \
		echo "  ${REPOSITORY}/$$image:latest"; \
		minikube image load ${REPOSITORY}/$$image:latest || exit 1; \
	done
	@echo "Images loaded!"

## k8s-deploy: applies the local overlay to the Minikube cluster
k8s-deploy:
	@echo "Deploying to kubernetes..."
	@test -f ${K8S_LOCAL}/secrets.env || cp ${K8S_LOCAL}/secrets.env.example ${K8S_LOCAL}/secrets.env
	- ${KUBECTL} -n anahita delete job migration-job --ignore-not-found
	${KUBECTL} apply -k ${K8S_LOCAL}
	@echo "Done! Watch it come up with: ${KUBECTL} -n anahita get pods"

## k8s-tunnel: serves the cluster's ingress at http://localhost (stop docker-compose first, it uses the same port)
k8s-tunnel:
	@echo "Tunneling to kubernetes..."
	sudo minikube tunnel

## k8s-delete: removes the local overlay from the cluster, data included
k8s-delete:
	@echo "Deleting from kubernetes..."
	${KUBECTL} delete -k ${K8S_LOCAL}
	@echo "Done!"

## k8s-dashboard: opens the Minikube dashboard
k8s-dashboard:
	minikube dashboard

## k8s-stop: stops the Minikube cluster, keeping its data
k8s-stop:
	minikube stop

## k8s-cleanup: stops and deletes the Minikube cluster
k8s-cleanup:
	minikube stop
	minikube delete

## test: runs the unit tests across the whole workspace. anahita-libs
## carries the scope drift guard, which fails on any HasScope("literal")
## anywhere in the workspace — run this before shipping a permission
## change.
##
## There is exactly one `test` target. There were briefly two, and make
## silently takes the last definition, so the comprehensive one never ran
## and `make test` was covering two modules out of twenty.
##
## Not `-v`: twenty modules of verbose output buries the one line that
## matters. Add it per-module when something fails.
test:
	@echo "Checking legacy/upgrade.sql statement order..."
	python3 migration-job/check_sql_order.py
	@echo "Testing anahita-libs..."
	@cd anahita-libs && go test ./...
	@cd ${AUTH_SRV} && go test ./...
	@cd ${AVATAR_SRV} && go test ./...
	@cd ${COMMENT_SRV} && go test ./...
	@cd ${COVER_SRV} && go test ./...
	@cd ${FEED_SRV} && go test ./...
	@cd ${FILE_SRV} && go test ./...
	@cd ${GRAPH_SRV} && go test ./...
	@cd ${GROUP_SRV} && go test ./...
	@cd ${HASHTAG_SRV} && go test ./...
	@cd ${LIKE_SRV} && go test ./...
	@cd ${LOCATION_SRV} && go test ./...
	@cd ${MAIL_SRV} && go test ./...
	@cd ${NOTIFICATION_SRV} && go test ./...
	@cd ${PERSON_SRV} && go test ./...
	@cd ${PHOTO_SRV} && go test ./...
	@cd ${SEARCH_SRV} && go test ./...
	@cd ${SOCIALGRAPH_SRV} && go test ./...
	@cd ${STORY_SRV} && go test ./...
	@cd ${TEXT_SRV} && go test ./...
	@echo "Tests passed!"
