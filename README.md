# cookd-api

Backend for **cookd**, a cooking coach you learn from: tell it what's in your fridge, send a photo,
and talk it through. Answers stream from Claude (`claude-sonnet-5`).

Written in Go against the standard library; the Anthropic SDK, `modernc.org/sqlite` (pure Go, so no
CGO) and `golang.org/x/time/rate` are the only dependencies.

## API

[`openapi.yaml`](openapi.yaml) is the contract. Every failure answers with `{ "error": string }`.

| Endpoint                            | What it does                                                    |
| ----------------------------------- | --------------------------------------------------------------- |
| `POST /auth/login`                  | Trade the admin password for a 30-day session token             |
| `POST /auth/google`                 | Same, with a Google ID token (optional, off by default)         |
| `GET /me`                           | The signed-in account                                           |
| `GET/POST /conversations`           | List, or start, conversations                                   |
| `GET/DELETE /conversations/{id}`    | One conversation, or delete it with its messages and photos     |
| `GET /conversations/{id}/messages`  | The messages, photos listed by id                               |
| `POST /conversations/{id}/messages` | Send text and/or photos; the answer streams back as SSE         |
| `GET /images/{id}`                  | A photo's bytes                                                 |
| `GET/POST /ingredients`             | Your pantry; POST takes a pasted list and switches items on     |
| `PATCH/DELETE /ingredients/{id}`    | Switch an ingredient on or off, or remove it                    |
| `GET/POST /journal`                 | Dishes you cooked: taste 1–5, minutes taken, a note             |
| `DELETE /journal/{id}`              | Delete an entry                                                 |

### Signing in

Both routes return the API's own signed token, sent as `Authorization: Bearer …` from then on. The UI
and API live on different sites, so there are no cookies. Each route exists only if it is configured.

- **Password** (`ADMIN_PASSWORD`): the UI posts the password to `/auth/login` and is signed in as the
  first address in `ALLOWED_EMAILS`. Attempts are limited per address, and the password must be at
  least 12 characters — a long random one is best.
- **Google** (`GOOGLE_CLIENT_ID`): the UI posts a Google ID token to `/auth/google`. The server checks
  Google's signature, the audience and that the email is in `ALLOWED_EMAILS`.

The allow-list is re-checked on every request: drop an address from `ALLOWED_EMAILS`, restart, and its
tokens stop working. Changing `SESSION_SECRET` signs everyone out.

### Streaming

`POST /conversations/{id}/messages` answers with Server-Sent Events:

```
event: delta
data: {"text":"Crack the eggs into a "}

event: done
data: {"messageId":2,"title":"I have eggs and cheese","stopReason":"end_turn"}
```

An `error` event replaces `done` on failure. The exchange is stored only once the answer is complete.

### What the model remembers

Every message reaches the model with your **pantry** (what's in stock, what's out) and your **15 most
recent journal entries**, read from the database at send time:

```
<pantry>
Available: eggs, rice
Out of stock: milk
</pantry>
<journal>
Dishes cooked before, newest first (today is 2026-09-20; taste is out of 5):
- 2026-09-14 Omelette: taste 4/5, took 15 min. too salty
</journal>
```

The block is attached to the latest message only and never stored in the conversation, so the model
sees today's pantry once instead of every old version, and everything before the latest message stays
cacheable. Ingredients stay in the list when they run out; re-adding one switches it back on.

## Configuration

Every variable except `GOOGLE_CLIENT_ID` and `ADMIN_PASSWORD` is required, and at least one of those
two must be set. The server refuses to start otherwise and names everything that is missing. See
[`.env.example`](.env.example).

## Local development

```console
$ cp .env.example .env.development   # fill it in
$ make run
$ TOKEN=$(curl -s localhost:50001/auth/login -d '{"password": "…"}' | jq -r .token)
$ ID=$(curl -s -X POST -H "Authorization: Bearer $TOKEN" localhost:50001/conversations | jq -r .id)
$ curl -N -X POST localhost:50001/conversations/$ID/messages \
    -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
    -d '{"text": "I have eggs, rice and a sad tomato. Dinner?"}'
```

`make` runs formatting, lint, vet and the tests — what CI runs.

## Deployment

Pull requests run `fmt-check`, `lint`, `vet` and `test` through
[pipes](https://github.com/dragunovartem99/pipes). Merging to `main` (or a manual run from the Actions
tab) runs the same checks, then pipes `deploy-vps` **builds the binary on GitHub's runner**, uploads it
to the VPS with rsync, resets the checkout there to the commit and runs [`deploy.sh`](deploy.sh). The SQLite driver is a very large pure-Go
package, so compiling it on the VPS took minutes; the `Dockerfile` now only copies the finished binary
into a distroless image. Caddy serves `cookd.dragunov.dev` and proxies to the container on
`127.0.0.1:50001`, and the database lives in the `cookd-data` Docker volume.

The workflow needs the repository secrets `VPS_HOST`, `VPS_USER`, `VPS_SSH_KEY` and
`VPS_PROJECT_PATH`, and the project directory on the VPS needs a `.env` with the variables above
(`ALLOWED_ORIGIN` is the UI's origin, e.g. `https://dragunovartem99.github.io`). The binary is built
for linux/amd64; change `GOARCH` in the workflow if the server is ARM.
