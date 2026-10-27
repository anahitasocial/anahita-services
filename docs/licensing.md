# Licensing

Anahita's code is open source. How Anahita works is copyleft, so that it
stays open wherever it is run; how to talk to it is permissive, so that
anybody can build on it, in any language, under any licence.

| Part | Licence | File |
| --- | --- | --- |
| The services, `anahita-libs`, and everything else in this repository but the protocol definitions and the examples | GNU Affero General Public License, version 3 only (`AGPL-3.0-only`) | [`LICENSE`](../LICENSE) |
| The gRPC and protocol buffer definitions in `anahita-libs/proto` (22 `.proto` files), and the code generated from them | Apache License 2.0 (`Apache-2.0`) | [`anahita-libs/proto/LICENSE`](../anahita-libs/proto/LICENSE) |
| The examples in `examples/`, made to be copied | Apache License 2.0 | [`examples/LICENSE`](../examples/LICENSE) |
| The web app, [anahita-web](https://github.com/anahitasocial/anahita-web) | MIT | its `LICENSE` |

Copyright 2008–2026 Rastin Mehr Design Studio Inc. (rmd studio). This page
explains the licences; it is not legal advice, and the licence texts are
what count.

## Running a site

Nothing is owed for running Anahita as it comes: a school, a newsroom or a
group of friends can install it, use it and charge for hosting it, and
publish nothing.

A site that **changes the services** and lets people use them over the
network must offer those people the source of its changed version, under
the same licence (section 13 of the AGPL). A link to a public repository
with the changes is enough. Changes kept to a developer's own machine or
test server, used by nobody else, owe nothing. Settings (`config.env`,
`secrets.env`) are not changes to the services, and the web app is under
its own licence, MIT, so a site's theme and texts in it are its own.

## Building on Anahita

| What you build | What it may be |
| --- | --- |
| A client: a mobile app, a bot, a script, another web app, talking to the API | Any licence. It is a separate program |
| A service of your own that calls Anahita's services over gRPC, in any language, with code generated from the definitions | Any licence, open or closed: the definitions are Apache 2.0. See [Services over gRPC](grpc.md) |
| A service of your own that answers in place of one of Anahita's, from the same definitions | Any licence, for the same reason |
| A service that uses `anahita-libs` (its permissions, federation, entities, clients), or a changed copy of one of Anahita's services | AGPL-3.0, offered to the people who use it |

The line runs between how to talk to Anahita, which anybody may use, and how
Anahita works, which stays open wherever it runs: nobody can take Anahita's
own code, change it, and run it closed for others.

## The name and the logo

Neither licence grants the use of the name Anahita, a registered trademark
of Rastin Mehr Design Studio Inc. in Canada, or of its logo, except to say
where the software comes from. A site built with Anahita has its own name.

## Other terms

rmd studio holds the copyright in all of Anahita's code, so it can license
any of it on other terms, or more permissively later. None are offered
today.
