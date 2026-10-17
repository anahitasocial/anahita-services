REPOSITORY=anahita
DOCKERFILES=dockerfiles

ABUSE_REPORT_BINARY=app-binary
ABUSE_REPORT_VERSION=latest
ABUSE_REPORT_SRV=abuse-report-service
RECOMMENDATION_BINARY=app-binary
RECOMMENDATION_VERSION=latest
RECOMMENDATION_SRV=recommendation-service
LINK_PREVIEW_BINARY=app-binary
LINK_PREVIEW_VERSION=latest
LINK_PREVIEW_SRV=link-preview-service
PULSE_BINARY=app-binary
PULSE_VERSION=latest
PULSE_SRV=pulse-service
VIDEO_BINARY=app-binary
VIDEO_WORKER_BINARY=worker-binary
VIDEO_VERSION=latest
VIDEO_SRV=video-service
VIDEO_WORKER=video-worker
ACTIVITYPUB_BINARY=app-binary
ACTIVITYPUB_VERSION=latest
ACTIVITYPUB_SRV=activitypub-service
ATPROTO_BINARY=app-binary
ATPROTO_VERSION=latest
ATPROTO_SRV=atproto-service

EXPORT_BINARY=app-binary
EXPORT_WORKER_BINARY=worker-binary
EXPORT_VERSION=latest
EXPORT_SRV=export-service
EXPORT_WORKER=export-worker
WEB_SHELL_BINARY=app-binary
WEB_SHELL_VERSION=latest
WEB_SHELL_SRV=web-shell

AUTH_BINARY=app-binary
AUTH_VERSION=latest
AUTH_SRV=auth-service


GRAPH_BINARY=app-binary
GRAPH_VERSION=latest
GRAPH_SRV=graph-grpc-service
SWEEPER_JOB=sweeper-job

ACTOR_BINARY=app-binary
ACTOR_VERSION=latest
ACTOR_SRV=actor-service

LIKE_BINARY=app-binary
LIKE_VERSION=latest
LIKE_SRV=like-service

NGINX_VERSION=latest
NGINX_SRV=nginx

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


NOTIFICATION_BINARY=app-binary
NOTIFICATION_VERSION=latest
NOTIFICATION_SRV=notification-service

FILE_BINARY=app-binary
FILE_VERSION=latest
FILE_SRV=file-grpc-service

IDENTITY_IMAGE_BINARY=app-binary
IDENTITY_IMAGE_VERSION=latest
IDENTITY_IMAGE_SRV=identity-image-service

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
build_dockerfiles: check_docker build_abuse_report build_recommendation build_link_preview build_pulse build_video build_export build_activitypub build_atproto build_auth build_graph build_sweeper build_actor build_like build_text build_photo build_hashtag build_location build_search build_socialgraph build_notification build_file build_identity_image build_mail build_feed
	@echo "Building dockerfiles..."
	docker build -f ${DOCKERFILES}/${ABUSE_REPORT_SRV}.dockerfile -t ${REPOSITORY}/${ABUSE_REPORT_SRV}:${ABUSE_REPORT_VERSION} .
	docker build -f ${DOCKERFILES}/${RECOMMENDATION_SRV}.dockerfile -t ${REPOSITORY}/${RECOMMENDATION_SRV}:${RECOMMENDATION_VERSION} .
	docker build -f ${DOCKERFILES}/${LINK_PREVIEW_SRV}.dockerfile -t ${REPOSITORY}/${LINK_PREVIEW_SRV}:${LINK_PREVIEW_VERSION} .
	docker build -f ${DOCKERFILES}/${PULSE_SRV}.dockerfile -t ${REPOSITORY}/${PULSE_SRV}:${PULSE_VERSION} .
	docker build -f ${DOCKERFILES}/${VIDEO_SRV}.dockerfile -t ${REPOSITORY}/${VIDEO_SRV}:${VIDEO_VERSION} .
	docker build -f ${DOCKERFILES}/${VIDEO_WORKER}.dockerfile -t ${REPOSITORY}/${VIDEO_WORKER}:${VIDEO_VERSION} .
	docker build -f ${DOCKERFILES}/${EXPORT_SRV}.dockerfile -t ${REPOSITORY}/${EXPORT_SRV}:${EXPORT_VERSION} .
	docker build -f ${DOCKERFILES}/${ACTIVITYPUB_SRV}.dockerfile -t ${REPOSITORY}/${ACTIVITYPUB_SRV}:${ACTIVITYPUB_VERSION} .
	docker build -f ${DOCKERFILES}/${ATPROTO_SRV}.dockerfile -t ${REPOSITORY}/${ATPROTO_SRV}:${ATPROTO_VERSION} .
	docker build -f ${DOCKERFILES}/${EXPORT_WORKER}.dockerfile -t ${REPOSITORY}/${EXPORT_WORKER}:${EXPORT_VERSION} .
	docker build -f ${DOCKERFILES}/${AUTH_SRV}.dockerfile -t ${REPOSITORY}/${AUTH_SRV}:${AUTH_VERSION} .
	docker build -f ${DOCKERFILES}/${GRAPH_SRV}.dockerfile -t ${REPOSITORY}/${GRAPH_SRV}:${GRAPH_VERSION} .
	docker build -f ${DOCKERFILES}/${SWEEPER_JOB}.dockerfile -t ${REPOSITORY}/${SWEEPER_JOB}:${GRAPH_VERSION} .
	docker build -f ${DOCKERFILES}/${ACTOR_SRV}.dockerfile -t ${REPOSITORY}/${ACTOR_SRV}:${ACTOR_VERSION} .
	docker build -f ${DOCKERFILES}/${LIKE_SRV}.dockerfile -t ${REPOSITORY}/${LIKE_SRV}:${LIKE_VERSION} .
	docker build -f ${DOCKERFILES}/${TEXT_SRV}.dockerfile -t ${REPOSITORY}/${TEXT_SRV}:${TEXT_VERSION} .
	docker build -f ${DOCKERFILES}/${PHOTO_SRV}.dockerfile -t ${REPOSITORY}/${PHOTO_SRV}:${PHOTO_VERSION} .
	docker build -f ${DOCKERFILES}/${HASHTAG_SRV}.dockerfile -t ${REPOSITORY}/${HASHTAG_SRV}:${HASHTAG_VERSION} .
	docker build -f ${DOCKERFILES}/${LOCATION_SRV}.dockerfile -t ${REPOSITORY}/${LOCATION_SRV}:${LOCATION_VERSION} .
	docker build -f ${DOCKERFILES}/${NGINX_SRV}.dockerfile -t ${REPOSITORY}/${NGINX_SRV}:${NGINX_VERSION} .
	docker build -f ${DOCKERFILES}/${SEARCH_SRV}.dockerfile -t ${REPOSITORY}/${SEARCH_SRV}:${SEARCH_VERSION} .
	docker build -f ${DOCKERFILES}/${SOCIALGRAPH_SRV}.dockerfile -t ${REPOSITORY}/${SOCIALGRAPH_SRV}:${SOCIALGRAPH_VERSION} .
	docker build -f ${DOCKERFILES}/${NOTIFICATION_SRV}.dockerfile -t ${REPOSITORY}/${NOTIFICATION_SRV}:${NOTIFICATION_VERSION} .
	docker build -f ${DOCKERFILES}/${FILE_SRV}.dockerfile -t ${REPOSITORY}/${FILE_SRV}:${FILE_VERSION} .
	docker build -f ${DOCKERFILES}/${IDENTITY_IMAGE_SRV}.dockerfile -t ${REPOSITORY}/${IDENTITY_IMAGE_SRV}:${IDENTITY_IMAGE_VERSION} .
	docker build -f ${DOCKERFILES}/${MAIL_SRV}.dockerfile -t ${REPOSITORY}/${MAIL_SRV}:${MAIL_VERSION} .
	docker build -f ${DOCKERFILES}/${MIGRATION_JOB}.dockerfile -t ${REPOSITORY}/${MIGRATION_JOB}:${MIGRATION_VERSION} .
	docker build -f ${DOCKERFILES}/${FEED_SRV}.dockerfile -t ${REPOSITORY}/${FEED_SRV}:${FEED_VERSION} .
	@echo "Done!"

## push_dockerfiles: pushes tagged versions to docker hub
push_dockerfiles: check_docker build_dockerfiles
	docker push ${REPOSITORY}/${ABUSE_REPORT_SRV}:${ABUSE_REPORT_VERSION}
	docker push ${REPOSITORY}/${RECOMMENDATION_SRV}:${RECOMMENDATION_VERSION}
	docker push ${REPOSITORY}/${LINK_PREVIEW_SRV}:${LINK_PREVIEW_VERSION}
	docker push ${REPOSITORY}/${PULSE_SRV}:${PULSE_VERSION}
	docker push ${REPOSITORY}/${VIDEO_SRV}:${VIDEO_VERSION}
	docker push ${REPOSITORY}/${VIDEO_WORKER}:${VIDEO_VERSION}
	docker push ${REPOSITORY}/${EXPORT_SRV}:${EXPORT_VERSION}
	docker push ${REPOSITORY}/${ACTIVITYPUB_SRV}:${ACTIVITYPUB_VERSION}
	docker push ${REPOSITORY}/${ATPROTO_SRV}:${ATPROTO_VERSION}
	docker push ${REPOSITORY}/${EXPORT_WORKER}:${EXPORT_VERSION}
	docker push ${REPOSITORY}/${AUTH_SRV}:${AUTH_VERSION}
	docker push ${REPOSITORY}/${GRAPH_SRV}:${GRAPH_VERSION}
	docker push ${REPOSITORY}/${SWEEPER_JOB}:${GRAPH_VERSION}
	docker push ${REPOSITORY}/${ACTOR_SRV}:${ACTOR_VERSION}
	docker push ${REPOSITORY}/${LIKE_SRV}:${LIKE_VERSION}
	docker push ${REPOSITORY}/${TEXT_SRV}:${TEXT_VERSION}
	docker push ${REPOSITORY}/${PHOTO_SRV}:${PHOTO_VERSION}
	docker push ${REPOSITORY}/${HASHTAG_SRV}:${HASHTAG_VERSION}
	docker push ${REPOSITORY}/${LOCATION_SRV}:${LOCATION_VERSION}
	docker push ${REPOSITORY}/${NGINX_SRV}:${NGINX_VERSION}
	docker push ${REPOSITORY}/${SEARCH_SRV}:${SEARCH_VERSION}
	docker push ${REPOSITORY}/${SOCIALGRAPH_SRV}:${SOCIALGRAPH_VERSION}
	docker push ${REPOSITORY}/${NOTIFICATION_SRV}:${NOTIFICATION_VERSION}
	docker push ${REPOSITORY}/${FILE_SRV}:${FILE_VERSION}
	docker push ${REPOSITORY}/${IDENTITY_IMAGE_SRV}:${IDENTITY_IMAGE_VERSION}
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

## build_actor: builds the actor binary (people and groups) as a linux executable
build_actor:
	@echo "Building actor binary.."
	cd ${ACTOR_SRV} && env GOOS=$(GOOS) GOARCH=$(GOARCH) CGO_ENABLED=0 go build -o ${ACTOR_BINARY} ./cmd/api
	@echo "Actor binary built!"

## build_like: builds the like binary as a linux executable
build_like:
	@echo "Building like binary.."
	cd ${LIKE_SRV} && env GOOS=$(GOOS) GOARCH=$(GOARCH) CGO_ENABLED=0 go build -o ${LIKE_BINARY} ./cmd/api
	@echo "Likes binary built!"

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

## build_recommendation: builds the recommendation binary as a linux executable
build_recommendation:
	@echo "Building recommendation binary.."
	cd ${RECOMMENDATION_SRV} && env GOOS=$(GOOS) GOARCH=$(GOARCH) CGO_ENABLED=0 go build -o ${RECOMMENDATION_BINARY} ./cmd/api
	@echo "Recommendation binary built!"

## build_activitypub: builds the activitypub binary as a linux executable
build_activitypub:
	@echo "Building activitypub binary.."
	cd ${ACTIVITYPUB_SRV} && env GOOS=$(GOOS) GOARCH=$(GOARCH) CGO_ENABLED=0 go build -o ${ACTIVITYPUB_BINARY} ./cmd/api
	@echo "Activitypub binary built!"

## build_atproto: builds the atproto binary as a linux executable
build_atproto:
	@echo "Building atproto binary.."
	cd ${ATPROTO_SRV} && env GOOS=$(GOOS) GOARCH=$(GOARCH) CGO_ENABLED=0 go build -o ${ATPROTO_BINARY} ./cmd/api
	@echo "Atproto binary built!"

## build_export: builds the export service and its worker as linux executables
build_export:
	@echo "Building export binaries.."
	cd ${EXPORT_SRV} && env GOOS=$(GOOS) GOARCH=$(GOARCH) CGO_ENABLED=0 go build -o ${EXPORT_BINARY} ./cmd/api
	cd ${EXPORT_SRV} && env GOOS=$(GOOS) GOARCH=$(GOARCH) CGO_ENABLED=0 go build -o ${EXPORT_WORKER_BINARY} ./cmd/worker
	@echo "Export binaries built!"

## build_video: builds the video service and its worker as linux executables
build_video:
	@echo "Building video binaries.."
	cd ${VIDEO_SRV} && env GOOS=$(GOOS) GOARCH=$(GOARCH) CGO_ENABLED=0 go build -o ${VIDEO_BINARY} ./cmd/api
	cd ${VIDEO_SRV} && env GOOS=$(GOOS) GOARCH=$(GOARCH) CGO_ENABLED=0 go build -o ${VIDEO_WORKER_BINARY} ./cmd/worker
	@echo "Video binaries built!"

## build_link_preview: builds the link preview binary as a linux executable
build_link_preview:
	@echo "Building link preview binary.."
	cd ${LINK_PREVIEW_SRV} && env GOOS=$(GOOS) GOARCH=$(GOARCH) CGO_ENABLED=0 go build -o ${LINK_PREVIEW_BINARY} ./cmd/api
	@echo "Link preview binary built!"

## build_pulse: builds the pulse binary as a linux executable
build_pulse:
	@echo "Building pulse binary.."
	cd ${PULSE_SRV} && env GOOS=$(GOOS) GOARCH=$(GOARCH) CGO_ENABLED=0 go build -o ${PULSE_BINARY} ./cmd/api
	@echo "Pulse binary built!"

## build_web_shell: builds the web shell binary as a linux executable. WEB_BUILD=<folder> copies the web app's build in beside it, for an image with the app baked in
build_web_shell:
	@echo "Building web shell binary.."
	cd ${WEB_SHELL_SRV} && env GOOS=$(GOOS) GOARCH=$(GOARCH) CGO_ENABLED=0 go build -o ${WEB_SHELL_BINARY} ./cmd/api
	@if [ -n "$(WEB_BUILD)" ]; then \
		test -f "$(WEB_BUILD)/index.html" || { echo "no index.html in $(WEB_BUILD)"; exit 1; }; \
		find ${WEB_SHELL_SRV}/web -mindepth 1 ! -name README.md -delete; \
		cp -R "$(WEB_BUILD)"/. ${WEB_SHELL_SRV}/web/; \
		echo "Copied the web app from $(WEB_BUILD)"; \
	fi
	@echo "Web shell binary built!"

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

## build_identity_image: builds the identity image binary (avatars and covers) as a linux executable
build_identity_image:
	@echo "Building identity image binary.."
	cd ${IDENTITY_IMAGE_SRV} && env GOOS=$(GOOS) GOARCH=$(GOARCH) CGO_ENABLED=0 go build -o ${IDENTITY_IMAGE_BINARY} ./cmd/api
	@echo "Identity image binary built!"

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
# up_build: build_auth build_broker build_listener build_logger build_mail
up_build: build_abuse_report build_recommendation build_link_preview build_pulse build_video build_export build_activitypub build_atproto build_auth build_graph build_actor build_like build_text build_photo build_hashtag build_location build_search build_socialgraph build_notification build_file build_identity_image build_mail build_feed
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

## graph: stops graph-grpc-service, removes docker image, builds service, and starts it
graph: build_graph
	@echo "Building ${GRAPH_SRV} docker image..."
	- docker-compose stop ${GRAPH_SRV}
	- docker-compose rm -f ${GRAPH_SRV}
	docker-compose up --build -d ${GRAPH_SRV}
	docker-compose start ${GRAPH_SRV}
	@echo "${GRAPH_SRV} built and started!"

## actor: stops actor-service (people and groups), removes docker image, builds service, and starts it
actor: build_actor
	@echo "Building ${ACTOR_SRV} docker image..."
	- docker-compose stop ${ACTOR_SRV}
	- docker-compose rm -f ${ACTOR_SRV}
	docker-compose up --build -d ${ACTOR_SRV}
	docker-compose start ${ACTOR_SRV}
	@echo "${ACTOR_SRV} built and started!"

## like: stops like-service, removes docker image, builds service, and starts it
like: build_like
	@echo "Building ${LIKE_SRV} docker image..."
	- docker-compose stop ${LIKE_SRV}
	- docker-compose rm -f ${LIKE_SRV}
	docker-compose up --build -d ${LIKE_SRV}
	docker-compose start ${LIKE_SRV}
	@echo "${LIKE_SRV} built and started!"

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

## recommendation: stops recommendation-service, removes docker image, builds service, and starts it
recommendation: build_recommendation
	@echo "Building ${RECOMMENDATION_SRV} docker image..."
	- docker-compose stop ${RECOMMENDATION_SRV}
	- docker-compose rm -f ${RECOMMENDATION_SRV}
	docker-compose up --build -d ${RECOMMENDATION_SRV}
	docker-compose start ${RECOMMENDATION_SRV}
	@echo "${RECOMMENDATION_SRV} built and started!"

# What activitypub-service is told while the lab is up.
FEDI_LAB_ENV=PUBLIC_DOMAIN=anahita.test PUBLIC_WEB_URL=https://anahita.test PUBLIC_API_URL=https://api.anahita.test ACTIVITYPUB_CA_FILE=/fedi-lab-certs/ca.crt ACTIVITYPUB_LAB=on

## fedi_lab: starts the federation lab: https at anahita.test, api.anahita.test and gts.test, and a GoToSocial server to talk to. See docs/federation-lab.md
fedi_lab:
	@./scripts/fedi-lab-certs.sh
	docker-compose --profile fedi-lab up -d fedi-lab-tls gotosocial
	@# activitypub-service is started again as the lab's site: the names
	@# other servers are told, and the lab's authority as the one it trusts.
	${FEDI_LAB_ENV} docker-compose up -d --force-recreate ${ACTIVITYPUB_SRV}
	@echo "The lab is up. Add to /etc/hosts:  127.0.0.1 anahita.test api.anahita.test gts.test"

## fedi_lab_down: stops the federation lab, keeping what its GoToSocial holds
fedi_lab_down:
	- docker-compose --profile fedi-lab stop fedi-lab-tls gotosocial
	- docker-compose --profile fedi-lab rm -f fedi-lab-tls gotosocial
	@# activitypub-service goes back to being this machine's own site.
	- docker-compose up -d --force-recreate ${ACTIVITYPUB_SRV}

## fedi_lab_reset: stops the federation lab and empties its GoToSocial
fedi_lab_reset: fedi_lab_down
	- docker volume rm anahita-services_fedi-lab-gotosocial

## web_shell: builds the web shell and starts it. Not started by `up`: it needs the web app's build, given as WEB_BUILD=<folder> or WEB_BUILD_PATH in .env
web_shell: build_web_shell
	@echo "Building ${WEB_SHELL_SRV} docker image..."
	- docker-compose --profile web stop ${WEB_SHELL_SRV}
	- docker-compose --profile web rm -f ${WEB_SHELL_SRV}
	docker-compose --profile web up --build -d ${WEB_SHELL_SRV}
	@echo "${WEB_SHELL_SRV} built and started!"

## activitypub: stops activitypub-service, removes docker image, builds service, and starts it
activitypub: build_activitypub
	@echo "Building ${ACTIVITYPUB_SRV} docker image..."
	- docker-compose stop ${ACTIVITYPUB_SRV}
	- docker-compose rm -f ${ACTIVITYPUB_SRV}
	docker-compose up --build -d ${ACTIVITYPUB_SRV}
	@echo "${ACTIVITYPUB_SRV} built and started!"

## atproto: stops atproto-service, removes docker image, builds service, and starts it
atproto: build_atproto
	@echo "Building ${ATPROTO_SRV} docker image..."
	- docker-compose stop ${ATPROTO_SRV}
	- docker-compose rm -f ${ATPROTO_SRV}
	docker-compose up --build -d ${ATPROTO_SRV}
	@echo "${ATPROTO_SRV} built and started!"

## export: stops export-service and its worker, removes their images, builds and starts them
export: build_export
	@echo "Building ${EXPORT_SRV} and ${EXPORT_WORKER} docker images..."
	- docker-compose stop ${EXPORT_SRV} ${EXPORT_WORKER}
	- docker-compose rm -f ${EXPORT_SRV} ${EXPORT_WORKER}
	docker-compose up --build -d ${EXPORT_SRV} ${EXPORT_WORKER}
	@echo "${EXPORT_SRV} and ${EXPORT_WORKER} built and started!"

## video: stops video-service and its worker, removes their images, builds and starts them
video: build_video
	@echo "Building ${VIDEO_SRV} and ${VIDEO_WORKER} docker images..."
	- docker-compose stop ${VIDEO_SRV} ${VIDEO_WORKER}
	- docker-compose rm -f ${VIDEO_SRV} ${VIDEO_WORKER}
	docker-compose up --build -d ${VIDEO_SRV} ${VIDEO_WORKER}
	@echo "${VIDEO_SRV} and ${VIDEO_WORKER} built and started!"

## link_preview: stops link-preview-service, removes docker image, builds service, and starts it
link_preview: build_link_preview
	@echo "Building ${LINK_PREVIEW_SRV} docker image..."
	- docker-compose stop ${LINK_PREVIEW_SRV}
	- docker-compose rm -f ${LINK_PREVIEW_SRV}
	docker-compose up --build -d ${LINK_PREVIEW_SRV}
	docker-compose start ${LINK_PREVIEW_SRV}
	@echo "${LINK_PREVIEW_SRV} built and started!"

## pulse: stops pulse-service, removes docker image, builds service, and starts it
pulse: build_pulse
	@echo "Building ${PULSE_SRV} docker image..."
	- docker-compose stop ${PULSE_SRV}
	- docker-compose rm -f ${PULSE_SRV}
	docker-compose up --build -d ${PULSE_SRV}
	docker-compose start ${PULSE_SRV}
	@echo "${PULSE_SRV} built and started!"

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

## identity_image: stops identity-image-service (avatars and covers), removes docker image, builds service, and starts it
identity_image: build_identity_image
	@echo "Building ${IDENTITY_IMAGE_SRV} docker image..."
	- docker-compose stop ${IDENTITY_IMAGE_SRV}
	- docker-compose rm -f ${IDENTITY_IMAGE_SRV}
	docker-compose up --build -d ${IDENTITY_IMAGE_SRV}
	docker-compose start ${IDENTITY_IMAGE_SRV}
	@echo "${IDENTITY_IMAGE_SRV} built and started!"

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
	@cd && go clean
	@cd ${GRAPH_SRV} && rm -f ${GRAPH_BINARY}
	@cd ${GRAPH_SRV} && go clean
	@cd ${ACTOR_SRV} && rm -f ${ACTOR_BINARY}
	@cd ${ACTOR_SRV} && go clean
	@cd ${LIKE_SRV} && rm -f ${LIKE_BINARY}
	@cd ${LIKE_SRV} && go clean
	@cd ${TEXT_SRV} && rm -f ${TEXT_BINARY}
	@cd ${TEXT_SRV} && go clean
	@cd ${PHOTO_SRV} && rm -f ${PHOTO_BINARY}
	@cd ${PHOTO_SRV} && go clean
	@cd ${ABUSE_REPORT_SRV} && rm -f ${ABUSE_REPORT_BINARY}
	@cd ${ABUSE_REPORT_SRV} && go clean
	@cd ${RECOMMENDATION_SRV} && rm -f ${RECOMMENDATION_BINARY}
	@cd ${RECOMMENDATION_SRV} && go clean
	@cd ${LINK_PREVIEW_SRV} && rm -f ${LINK_PREVIEW_BINARY}
	@cd ${LINK_PREVIEW_SRV} && go clean
	@cd ${PULSE_SRV} && rm -f ${PULSE_BINARY}
	@cd ${PULSE_SRV} && go clean
	@cd ${VIDEO_SRV} && rm -f ${VIDEO_BINARY} ${VIDEO_WORKER_BINARY}
	@cd ${VIDEO_SRV} && go clean
	@cd ${EXPORT_SRV} && rm -f ${EXPORT_BINARY} ${EXPORT_WORKER_BINARY}
	@cd ${EXPORT_SRV} && go clean
	@cd ${ACTIVITYPUB_SRV} && rm -f ${ACTIVITYPUB_BINARY}
	@cd ${ACTIVITYPUB_SRV} && go clean
	@cd ${ATPROTO_SRV} && rm -f ${ATPROTO_BINARY}
	@cd ${ATPROTO_SRV} && go clean
	@cd ${WEB_SHELL_SRV} && rm -f ${WEB_SHELL_BINARY}
	@cd ${WEB_SHELL_SRV} && go clean
	@cd ${HASHTAG_SRV} && rm -f ${HASHTAG_BINARY}
	@cd ${HASHTAG_SRV} && go clean
	@cd ${LOCATION_SRV} && rm -f ${LOCATION_BINARY}
	@cd ${LOCATION_SRV} && go clean
	@cd ${SEARCH_SRV} && rm -f ${SEARCH_BINARY}
	@cd ${SEARCH_SRV} && go clean
	@cd ${SOCIALGRAPH_SRV} && rm -f ${SOCIALGRAPH_BINARY}
	@cd ${SOCIALGRAPH_SRV} && go clean
	@cd && go clean
	@cd ${NOTIFICATION_SRV} && rm -f ${NOTIFICATION_BINARY}
	@cd ${NOTIFICATION_SRV} && go clean
	@cd ${FILE_SRV} && rm -f ${FILE_BINARY}
	@cd ${FILE_SRV} && go clean
	@cd ${IDENTITY_IMAGE_SRV} && rm -f ${IDENTITY_IMAGE_BINARY}
	@cd ${IDENTITY_IMAGE_SRV} && go clean
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
K8S_IMAGES=${ABUSE_REPORT_SRV} ${RECOMMENDATION_SRV} ${LINK_PREVIEW_SRV} ${PULSE_SRV} ${VIDEO_SRV} ${VIDEO_WORKER} ${EXPORT_SRV} ${EXPORT_WORKER} ${ACTIVITYPUB_SRV} ${ATPROTO_SRV} ${AUTH_SRV} ${IDENTITY_IMAGE_SRV} ${FEED_SRV} ${FILE_SRV} ${GRAPH_SRV} ${ACTOR_SRV} ${HASHTAG_SRV} ${LIKE_SRV} ${LOCATION_SRV} ${MAIL_SRV} ${MIGRATION_JOB} ${NGINX_SRV} ${NOTIFICATION_SRV} ${PHOTO_SRV} ${SEARCH_SRV} ${SOCIALGRAPH_SRV} ${SWEEPER_JOB} ${TEXT_SRV}

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

## api_docs: writes the API reference, docs/api/, from the code: each
## described service's file from its routes, then anahita.json of them all.
## `make test` fails while what is committed there is not what the code says
## (see docs/api.md). Commit what this writes with the change that made it.
##
## A service is added here when its routes are described (anahita-libs/apidoc).
api_docs:
	@cd ${HASHTAG_SRV} && APIDOC_UPDATE=1 go test ./cmd/api/ -run TestTheAPIReference -count=1
	@cd ${SEARCH_SRV} && APIDOC_UPDATE=1 go test ./cmd/api/ -run TestTheAPIReference -count=1
	@cd ${LIKE_SRV} && APIDOC_UPDATE=1 go test ./cmd/api/ -run TestTheAPIReference -count=1
	@cd ${LINK_PREVIEW_SRV} && APIDOC_UPDATE=1 go test ./cmd/api/ -run TestTheAPIReference -count=1
	@cd ${PULSE_SRV} && APIDOC_UPDATE=1 go test ./cmd/api/ -run TestTheAPIReference -count=1
	@cd ${IDENTITY_IMAGE_SRV} && APIDOC_UPDATE=1 go test ./cmd/api/ -run TestTheAPIReference -count=1
	@cd ${NOTIFICATION_SRV} && APIDOC_UPDATE=1 go test ./cmd/api/ -run TestTheAPIReference -count=1
	@cd ${RECOMMENDATION_SRV} && APIDOC_UPDATE=1 go test ./cmd/api/ -run TestTheAPIReference -count=1
	@cd ${ABUSE_REPORT_SRV} && APIDOC_UPDATE=1 go test ./cmd/api/ -run TestTheAPIReference -count=1
	@cd ${LOCATION_SRV} && APIDOC_UPDATE=1 go test ./cmd/api/ -run TestTheAPIReference -count=1
	@cd ${ATPROTO_SRV} && APIDOC_UPDATE=1 go test ./cmd/api/ -run TestTheAPIReference -count=1
	@cd ${PHOTO_SRV} && APIDOC_UPDATE=1 go test ./cmd/api/ -run TestTheAPIReference -count=1
	@cd ${VIDEO_SRV} && APIDOC_UPDATE=1 go test ./cmd/api/ -run TestTheAPIReference -count=1
	@cd ${SOCIALGRAPH_SRV} && APIDOC_UPDATE=1 go test ./cmd/api/ -run TestTheAPIReference -count=1
	@cd ${FEED_SRV} && APIDOC_UPDATE=1 go test ./cmd/api/ -run TestTheAPIReference -count=1
	@cd ${ACTIVITYPUB_SRV} && APIDOC_UPDATE=1 go test ./cmd/api/ -run TestTheAPIReference -count=1
	@cd ${TEXT_SRV} && APIDOC_UPDATE=1 go test ./cmd/api/ -run TestTheAPIReference -count=1
	@cd ${ACTOR_SRV} && APIDOC_UPDATE=1 go test ./cmd/api/ -run TestTheAPIReference -count=1
	@cd ${AUTH_SRV} && APIDOC_UPDATE=1 go test ./cmd/api/ -run TestTheAPIReference -count=1
	@cd ${EXPORT_SRV} && APIDOC_UPDATE=1 go test ./cmd/api/ -run TestTheAPIReference -count=1
	@cd anahita-libs && APIDOC_UPDATE=1 go test ./apidoc/ -run TestTheCombinedReference -count=1

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
	@cd ${ABUSE_REPORT_SRV} && go test ./...
	@cd ${IDENTITY_IMAGE_SRV} && go test ./...
	@cd ${FEED_SRV} && go test ./...
	@cd ${FILE_SRV} && go test ./...
	@cd ${GRAPH_SRV} && go test ./...
	@cd ${ACTOR_SRV} && go test ./...
	@cd ${HASHTAG_SRV} && go test ./...
	@cd ${LIKE_SRV} && go test ./...
	@cd ${LOCATION_SRV} && go test ./...
	@cd ${MAIL_SRV} && go test ./...
	@cd ${NOTIFICATION_SRV} && go test ./...
	@cd ${PHOTO_SRV} && go test ./...
	@cd ${VIDEO_SRV} && go test ./...
	@cd ${EXPORT_SRV} && go test ./...
	@cd ${ACTIVITYPUB_SRV} && go test ./...
	@cd ${ATPROTO_SRV} && go test ./...
	@cd ${SEARCH_SRV} && go test ./...
	@cd ${SOCIALGRAPH_SRV} && go test ./...
	@cd ${RECOMMENDATION_SRV} && go test ./...
	@cd ${LINK_PREVIEW_SRV} && go test ./...
	@cd ${PULSE_SRV} && go test ./...
	@cd ${WEB_SHELL_SRV} && go test ./...
	@cd ${TEXT_SRV} && go test ./...
	@echo "Tests passed!"
