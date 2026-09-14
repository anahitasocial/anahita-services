# Anahita Microservices

- __Website:__ www.anahita.io
- __Founder & Architect:__ Rastin Mehr

# Architecture

Anahita is a social networking platform and framework

- Anahita uses a graph architecture where actors (people, groups, events, etc.), media (photos, text, videos), and tags (hashtags, locations) are all nodes and relationships amongst them are edges.
- Other data entities are stored outside the graph in separate database tables, for example: notifications, authentication data (logs, credentials, oauth, tokens, webauthn). 
- All design decisions should consider compatibilaty and scalibility with ATProtocol and ActivityPub and ability to easily join the fediverse. 

## Coding Guidelines

Please make sure that:

- The code is readable and easy to maintain by humans
- Commit messages aren't verbose, short, and human friendly
- Instead of `Alpha` and `Beta` releases, we use the terms `Embryo` and `Birth` releases. The main branch is always the Embryo; tagged releases are Birth releases.
- Use Star Trek names of characters and places when testing 
- Always ask first before committing and display the commit messages
- Never commit directly to the main branch, if you are on the main branch, always create a branch under issues/
