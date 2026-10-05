# Consuming mailpitter

mailpitter runs one shared mailpit container on your dev box. Point your
project at it instead of running your own.

## Prerequisites

- `mailpitter` installed and running (`mailpitter install`).
- Docker Compose v2.

## 1. Register your project

```
mailpitter register <name> --service <docker-compose-service>
```

Example:

```
mailpitter register taiga --service taiga-back
```

Add `--mail-vars laravel` if your stack uses Laravel-style `MAIL_*`
variables instead of Django-style `EMAIL_*`.

`mailpitter register` prints two snippets to stdout: `.env` vars and a
`docker-compose.yml` fragment. Apply both.

## 2. Apply the snippets

`.env`:

```
EMAIL_HOST=mailpit.test
EMAIL_PORT=1025
EMAIL_USE_TLS=False
EMAIL_USE_SSL=False
```

`docker-compose.yml`, under the service that sends mail:

```yaml
extra_hosts:
  - "mailpit.test:host-gateway"
```

If your project still defines its own `mailpit` service in
`docker-compose.yml`, remove it — the shared mailpitter container owns
ports 1025 and 8025 on `127.0.0.1`. On native Linux Docker Engine it also
publishes 1025 on the docker0 gateway, which is where `host-gateway` points.
With ufw active you need one allow rule; `mailpitter doctor` prints it. See
[Reaching SMTP from other containers](README.md#reaching-smtp-from-other-containers).

## 3. Verify

Bring your stack up and check the inbox:

```
make up                      # or however your project boots
mailpitter status            # confirm container + API are healthy
open https://mailpit.test    # UI
```

Trigger an email from your app; it should appear in the inbox at
`https://mailpit.test`.

## 4. Wire into your project's Makefile (optional)

Add `register`/`deregister` to your project's `up`/`down` targets so the
state stays clean:

```make
up:
	mailpitter register <name> --service <svc>
	docker compose up -d
	./bin/caddy-dev register

down:
	./bin/caddy-dev deregister
	docker compose down
	mailpitter deregister <name>
```

`mailpitter register` is idempotent (a no-op when the name + service haven't
changed). `mailpitter deregister` is idempotent too — unknown names exit 0
with a note on stderr.

## 5. Tear down

```
mailpitter deregister <name>
```

Revert the two file edits to fully detach. The shared mailpit keeps running
for other consumers.
