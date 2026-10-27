# Developing

[Documentation](README.md) › Developing

The pages for writing your own app or service against an Anahita site.

1. **The API.** How requests and answers are shaped, lists and paging,
   lengths, what the viewer may do, and refusals: [The API](api.md). Every
   route, with what it takes and sends, is in the API reference:
   `docs/api/index.html` ([The reference](api.md#the-reference)).
2. **Signing in.** An app signs people in with OAuth, and asks for the scopes
   it needs: [OAuth scopes and clients](oauth.md), and
   [Signing in](api.md#signing-in).
3. **How the site is put together.** The services, the routes, the graph of
   nodes and edges, and the shared library: [Architecture](architecture.md).
4. **Services a site may not run.** What an app is told when one is missing:
   [Services an installation may not run](api.md#services-an-installation-may-not-run).
5. **A service you can replace.** Recommendations keep a contract that
   another service can keep in their place:
   [The contract](recommendations.md#the-contract).
6. **The web app** as an example of a client:
   [its architecture](https://github.com/anahitasocial/anahita-web/blob/main/docs/architecture.md).
7. **Other networks.** What an Anahita site sends to the fediverse and
   Bluesky, and takes from them: [Federation](federation.md).
8. **Over gRPC, inside the cluster.** A service of your own, in any
   language, calling the graph, files and mail services directly:
   [Services over gRPC](grpc.md), with examples in Go,
   Node.js, Python, Ruby and PHP.
9. **The licences.** An app or a service of your own may have any licence:
   the API is open to any client, and the protocol buffer definitions are
   Apache 2.0; the services and `anahita-libs` are AGPL-3.0:
   [Licensing](licensing.md).

Not written yet:

1. **The events on the bus:** each event on the `anahita_events` exchange,
   what it carries, and who listens to it. Today
   [Architecture](architecture.md#components) says only that it is there.
2. **Writing a service of your own,** from an empty folder to running beside
   the others.
3. **Writing a client,** from registering it to its first signed-in request.
