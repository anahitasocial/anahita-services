# Anahita Microservices

- **Website:** www.anahita.io
- **Founder & Architect:** Rastin Mehr

## Architecture

Anahita is a social networking platform and framework.

- **Graph Architecture:** Anahita uses a graph architecture where actors (people, groups, events, etc.), media (photos, text, videos), and tags (hashtags, locations) are nodes. Relationships between these entities are represented as edges.
- **Separate Databases:** Other data entities, such as notifications, authentication data (logs, credentials, OAuth tokens, WebAuthn), are stored in separate database tables outside the graph.
- **Compatibility and Scalability:** All design decisions should prioritize compatibility and scalability with ATProtocol and ActivityPub, and facilitate easy integration into the fediverse.

## Coding Guidelines

Please adhere to these guidelines:

- **Readability and Maintainability:** Ensure that your code is readable and easy to maintain for human developers.
- **Commit Messages:** Keep commit messages concise, human-friendly, and informative.
- **Release Terminology:** Instead of using “Alpha” and “Beta” releases, we use “Embryo” and “Birth” releases. The main branch is always the Embryo, while tagged releases are Birth releases.
- **Testing Names:** Use Star Trek names of characters and places when testing.
- **Commit Before Committing:** Always ask for permission before committing and display the commit message.
- **Main Branch Practice:** If you are on the main branch, create a branch under “issues” before committing.
- **Documentation Updates:** After building new features or modifying the underlying architecture, update the documentation using human-friendly language to help developers quickly understand and learn it.
