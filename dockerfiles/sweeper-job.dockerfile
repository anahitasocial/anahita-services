# Account sweeper — purges profiles whose 30-day deletion grace period has
# expired.
#
# A one-shot job rather than a long-running service: it runs, reports how many
# accounts it purged, and exits. Scheduled as a CronJob in Kubernetes and
# available on demand through the compose "jobs" profile.
#
# Deliberately not folded into graph-grpc-service, which runs three replicas —
# an in-process loop there would put three concurrent sweeps on one queue.
FROM alpine:latest
RUN mkdir -p /app

# Copy binary
COPY graph-grpc-service/sweeper-binary /app/sweeper-binary

# Run once and exit. The exit code says whether the sweep completed.
CMD [ "/app/sweeper-binary" ]
