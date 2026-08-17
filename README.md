# anahita-services
Anahita as a microservices architecture

This is the source code Anahita written in Golang. This project
consists of a number of loosely coupled microservices, all written in Go:

<!--- - broker-service: an optional single entry point to connect to all services from one place (accepts JSON;
sends JSON, makes calls via gRPC, and pushes to RabbitMQ) -->
- authentication-service: authenticates users against a Postgres database (accepts JSON)
<!--- - logger-service: logs important events to a MongoDB database (accepts RPC, gRPC, and JSON) -->
<!--- - queue-listener-service: consumes messages from amqp (RabbitMQ) and initiates actions based on payload (sends via RPC) -->
<!--- - mail-service: sends email (accepts JSON) -->

All services (except the broker) register their access urls with etcd, and renew their leases automatically.
This allows us to implement a simple service discovery system, where all service URLs are accessible with
"service maps" in the Config type used to share application configuration in the broker service.

In addition to the microservices, the included `docker-compose.yml` at the root level of the project
starts the following services:

- MySQL - used by the authentication service to store user accounts
- Redis - used to store user sessions
<!--- - MongoDB - used by the logger service to save logs from all services -->
<!--- - etcd - used for service discovery -->
<!--- - mailhog - used as a fake mail server to work with the mail service -->

## Running the project
From the root level of the project, execute this command (this assumes that you have 
[GNU make](https://www.gnu.org/software/make/) and a recent version
of [Docker](https://www.docker.com/products/docker-desktop) installed on your machine):

~~~
make up_build 
~~~

If the code has not changed, subsequent runs can just be `make up`.

Then start the front end:

~~~
make start
~~~

Hit the front end with your web browser at `http://localhost:80`. You can also access a web 
front end to the logger service by going to `http://localhost:8082` (or whatever port you
specify in the `docker-compose.yml file`).

To stop everything:

~~~
make stop
make down
~~~

While working on code, you can rebuild just the service you are working on by
executing

`make auth`

Where `auth` is one of the services:

1. actor-grpc-service
1. article-service
1. auth-service
1. avatar-service
1. comment-service
1. cover-service
1. file-grpc-service
1. group-service
1. hashtag-service
1. like-service
1. location-service
1. mail-grpc-service
1. medium-grpc-service
1. note-service
1. notification-service
1. person-service
1. photo-service
1. search-service
1. socialgraph-service
1. story-service
1. topic-service

All make commands:

~~~
rastinmehr@mysteriusisland anahita-services % make help
 Choose a command:
  up                  starts all containers in the background without forcing build
  down                stop docker compose
  build_dockerfiles   builds all dockerfile images
  push_dockerfiles    pushes tagged versions to docker hub
  swarm_up            starts the swarm
  swarm_down          stops the swarm
  build_auth          builds the authentication binary as a linux executable
  up_build            stops docker-compose (if running), builds all projects and starts docker compose
  auth                stops authentication-service, removes docker image, builds service, and starts it
  test                runs all tests
  clean               runs go clean and deletes binaries
  help                displays help
~~~

# Resources

- Anahita nodes and entities examples on go playground: https://go.dev/play/p/96bs5YHt84x
