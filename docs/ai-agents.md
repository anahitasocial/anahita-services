# Anahita MCP

[Documentation](README.md) › Anahita MCP

Anahita's docs and API reference, in the AI agent you already use: Claude,
Gemini, Cursor, VS Code, or any other that takes an MCP server. The agent
then answers from the docs as they are today, not from what it remembers:
how to set up a site, how federation works, which route follows a profile
and what it answers.

The server is [anahita-mcp](https://github.com/anahitasocial/anahita-mcp),
open source under MIT. It reads, and only reads: these docs, the web app's,
and the API reference. There are two ways to use it.

| Way | Address or command | For |
| --- | --- | --- |
| Hosted, nothing to install | `https://mcp.anahita.io/mcp` | Chat apps, which reach servers over the internet, and anybody without Node.js |
| On your computer | `npx -y anahita-mcp` | Coding agents; your questions never leave your machine through it. Needs [Node.js](https://nodejs.org) 20 or later |

## Adding it

| Client | Hosted | On your computer |
| --- | --- | --- |
| Claude Code | `claude mcp add --transport http anahita https://mcp.anahita.io/mcp` | `claude mcp add anahita -- npx -y anahita-mcp` |
| Claude Desktop and claude.ai | Settings › Connectors › Add custom connector, with the address | Settings › Developer › Edit Config: in `mcpServers`, `"anahita": { "command": "npx", "args": ["-y", "anahita-mcp"] }`, then restart |
| Gemini CLI | `gemini mcp add --transport http anahita https://mcp.anahita.io/mcp` | `gemini mcp add anahita npx -y anahita-mcp` |
| Cursor | In `~/.cursor/mcp.json`, under `mcpServers`: `"anahita": { "url": "https://mcp.anahita.io/mcp" }` | `"anahita": { "command": "npx", "args": ["-y", "anahita-mcp"] }` |
| VS Code | In `.vscode/mcp.json`, under `servers`: `"anahita": { "type": "http", "url": "https://mcp.anahita.io/mcp" }` | `"anahita": { "type": "stdio", "command": "npx", "args": ["-y", "anahita-mcp"] }` |
| Any other | The address, as a remote MCP server (Streamable HTTP) | The command, as a local one (standard input and output) |

Then ask, for example: *"How does Anahita decide what an old post is
worth?"*, or *"Which API route lets a person follow a group, and what does
it send back?"*

## What it can do

| Tool | Answers |
| --- | --- |
| `search_docs` | The sections of the docs that match some words, best first, with where each is |
| `list_docs` | Every page, in its groups, with its sections |
| `read_doc` | A page, or one section of it |
| `list_endpoints` | The API's services; one service's routes; or the routes that match some words |
| `read_endpoint` | One route in full: who may call it, what it takes, every answer it gives |
| `read_shape` | One shape the API takes or answers with, and its fields |

Every answer gives the www.anahita.io address of what it read, so that the
agent can point you to the page.

## What it keeps

- **On your computer:** nothing is sent anywhere. It fetches two files from
  www.anahita.io, keeps them in your system's cache folder, and searches
  them there. It works offline with what it fetched last.
- **Hosted:** it keeps no record of the questions or of who asked. AWS keeps
  only when the function ran and for how long, for one day. It runs in
  Montreal (AWS ca-central-1).

The docs it reads are a day behind these at most: the website is rebuilt
on every change to it and every night.

## Without MCP

| File | What it holds |
| --- | --- |
| [`www.anahita.io/llms.txt`](https://www.anahita.io/llms.txt) | The site and its docs in brief, for language models |
| [`www.anahita.io/docs/index.json`](https://www.anahita.io/docs/index.json) | Every page of the docs as Markdown, cut into sections, in one file |
| [`www.anahita.io/api/anahita.json`](https://www.anahita.io/api/anahita.json) | The API reference, OpenAPI 3.1 |

## Your own copy

An operator can host the server for their own site's docs, on any server
(`npx -y anahita-mcp --http`) or on AWS Lambda:
[Hosting it](https://github.com/anahitasocial/anahita-mcp/blob/main/docs/hosting.md).

## Where the code is

| What | Where |
| --- | --- |
| The server | [anahita-mcp](https://github.com/anahitasocial/anahita-mcp): `src/server.ts` for the tools |
| The docs index it reads | anahita.io: `src/app/docs/index.json/route.js` |
| The API reference it reads | `docs/api/`, made from the code ([The API](api.md#the-reference)) |
