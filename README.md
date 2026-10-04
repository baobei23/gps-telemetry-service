<!-- Improved compatibility of back to top link: See: https://github.com/othneildrew/Best-README-Template/pull/73 -->

<a id="readme-top"></a>

<!-- PROJECT SHIELDS -->

[![Actions][actions-shield]][actions-url] [![GoDoc][godoc-shield]][godoc-url]
[![MIT License][license-shield]][license-url]
[![LinkedIn][linkedin-shield]][linkedin-url]

<br />
<div align="center">
  <h3 align="center">Goapp Gin Boilerplate</h3>

  <p align="center">
    An opinionated guideline to structure a Go web application/service.
    <br />
    <a href="https://github.com/baobei23/goapp"><strong>Explore the docs »</strong></a>
    <br />
    <br />
    <a href="https://github.com/baobei23/goapp">View Demo</a>
    ·
    <a href="https://github.com/baobei23/goapp/issues">Report Bug</a>
    ·
    <a href="https://github.com/baobei23/goapp/issues">Request Feature</a>
  </p>
</div>

<!-- ABOUT THE PROJECT -->

## About The Project

This is my personal boilerplate for building Go web applications. It serves as a
structured starting point, adapted from community best practices to implement
[DDD (Domain Driven Development)](https://en.wikipedia.org/wiki/Domain-driven_design)
and
[Clean Architecture](https://blog.cleancoder.com/uncle-bob/2012/08/13/the-clean-architecture.html).

The goal of this project is to speed up development by providing a solid
foundation with ready-to-use configurations for JWT, Docker, Kubernetes, and
Observability.

### Attribution

This project is a customized implementation of the awesome
[Goapp](https://github.com/naughtygopher/goapp) by
[naughtygopher](https://github.com/naughtygopher). It has been adapted to
include specific configuration patterns, Kubernetes manifests, JWT
authentication strategy, and other project-specific enhancements while retaining
the solid architectural core of the original.

### Built With

- [![Go](https://img.shields.io/badge/Go-00ADD8?style=for-the-badge&logo=go&logoColor=white)](https://go.dev/)
- [![Gin](https://img.shields.io/badge/Gin-008ECF?style=for-the-badge&logo=go&logoColor=white)](https://gin-gonic.com/)
- [![PostgreSQL](https://img.shields.io/badge/PostgreSQL-316192?style=for-the-badge&logo=postgresql&logoColor=white)](https://www.postgresql.org/)
- [![Docker](https://img.shields.io/badge/Docker-2496ED?style=for-the-badge&logo=docker&logoColor=white)](https://www.docker.com/)
- [![Kubernetes](https://img.shields.io/badge/Kubernetes-326CE5?style=for-the-badge&logo=kubernetes&logoColor=white)](https://kubernetes.io/)
- [![JWT](https://img.shields.io/badge/JWT-000000?style=for-the-badge&logo=JSON%20web%20tokens&logoColor=whit)](https://www.jwt.io)
- [![Prometheus](https://img.shields.io/badge/Prometheus-000000?style=for-the-badge&logo=prometheus&labelColor=000000)](https://prometheus.io)
- [![OpenTelemetry](https://img.shields.io/badge/OpenTelemetry-4f62ad?style=for-the-badge&logo=opentelemetry&logoColor=white)](https://opentelemetry.io)

<p align="right">(<a href="#readme-top">back to top</a>)</p>

This guideline works for 1.4+ (i.e. since introduction of the structure is
explained based on a note taking web application.

<p align="right">(<a href="#readme-top">back to top</a>)</p>

## Environment Variables

This application requires several environment variables to run correctly across
local development, Docker Compose, and Kubernetes.

### Required

- `ENV` - application environment (`local`, `test`, `staging`, `production`)
- `APP_NAME` - service name
- `APP_VERSION` - application version

- `POSTGRES_HOST` - database host (`localhost` for local, `postgres` for Docker
  Compose service-to-service)
- `POSTGRES_PORT` - database port (typically `5432`)
- `POSTGRES_STORENAME` - database name
- `POSTGRES_USERNAME` - database username
- `POSTGRES_PASSWORD` - database password
- `POSTGRES_SSLMODE` - PostgreSQL SSL mode (commonly `disable` in local
  development)

- `JWT_SECRET` - secret used to sign and verify JWTs

  > Keep this name consistent across all environments (the code currently reads
  > `JWT_SECRET`).

### Optional

- `ENABLE_METRICS` - enable/disable metrics (`true`/`false`, enabled by default)
- `ENABLE_TRACING` - enable/disable tracing (`true`/`false`, enabled by default)

### Example (`.envrc`)

export ENV= export APP_NAME= export APP_VERSION=

export POSTGRES_HOST= export POSTGRES_PORT= export POSTGRES_STORENAME= export
POSTGRES_USERNAME= export POSTGRES_PASSWORD= export POSTGRES_SSLMODE=

export JWT_SECRET_KEY=

export ENABLE_METRICS= export ENABLE_TRACING=

## Table of contents

1. [Directory structure](#directory-structure)
2. [Configs package](#internalconfigs)
3. [API package](#internalapi)
4. [Users](#internalusers) (would be common for all such business logic / domain
   units, 'usernotes' being similar to users) package.
5. [Testing](#internalusers_test)
6. [pkg package](#internalpkg) (datastore, logger)
7. [HTTP server](#internalhttp) (templates)
8. [docker](#docker)
9. [db](#db)
10. [main.go](#maingo)
11. [Error handling](#error-handling)
12. [Dependency flow](#dependency-flow)

<hr>

## Directory structure

```bash
├── cmd
│   ├── server
│   │   ├── grpc
│   │   │   └── grpc.go
│   │   └── http
│   │       ├── handlers.go
│   │       ├── handlers_auth.go
│   │       ├── handlers_usernotes.go
│   │       ├── handlers_users.go
│   │       ├── http.go
│   │       ├── middlewares.go
│   │       └── web
│   │           └── templates
│   │               └── index.html
│   └── subscribers
│       └── kafka
│           └── kafka.go
├── db
│   └── migrations
│       └── 000001_initial_schema.up.sql
├── docker
│   ├── docker-compose.yml
│   └── Dockerfile
├── go.mod
├── go.sum
├── internal
│   ├── api
│   │   ├── api.go
│   │   ├── usernotes.go
│   │   └── users.go
│   ├── configs
│   │   └── configs.go
│   ├── pkg
│   │   ├── apm
│   │   │   ├── apm.go
│   │   │   ├── grpc.go
│   │   │   ├── http.go
│   │   │   ├── meter.go
│   │   │   ├── prometheus.go
│   │   │   └── tracer.go
│   │   ├── health
│   │   │   ├── depprober.go
│   │   │   ├── health.go
│   │   │   └── http.go
│   │   ├── jwt
│   │   │   └── jwt.go
│   │   ├── logger
│   │   │   ├── default.go
│   │   │   └── logger.go
│   │   ├── postgres
│   │   │   └── postgres.go
│   │   └── sysignals
│   │       └── sysignals.go
│   ├── usernotes
│   │   ├── store_postgres.go
│   │   └── usernotes.go
│   └── users
│       ├── store_postgres.go
│       └── users.go
├── k8s
│   ├── app.yaml
│   ├── app-config.yaml
│   ├── postgres.yaml
│   └── secrets.yaml
├── LICENSE
├── main.go
├── inits.go
├── shutdown.go
├── README.md
```

<p align="right">(<a href="#readme-top">back to top</a>)</p>

## internal

["internal" is a special directory name in Go](https://go.dev/doc/go1.4#internalpackages),
wherein any exported name/entity can only be consumed within its immediate
parent or any other packages within internal directory.

## internal/configs

Creating a dedicated configs package might seem like an overkill, but it makes
things easier. In the app, you see the HTTP configs are hardcoded and returned.
Later you decide to change to consume from env variables. All you do is update
the configs package. And further down the line, maybe you decide to introduce
something like [etcd](https://github.com/etcd-io/etcd), then you define the
dependency in `Configs` and update the functions accordingly. This is yet
another separation of concern package, to try and keep `main` tidy.

## internal/api

The API package is supposed to have all the APIs _*exposed*_ by the application.
A dedicated API package is created to standardize the functionality, when there
are different kinds of services running. e.g. an HTTP & a gRPC server, a Kafka &
Pubsub subscriber etc. In such cases, the respective "handler" functions would
inturn call `api.<Method name>`. This gives a guarantee that all your APIs
behave exactly the same without any accidental inconsistencies across different
I/O methods. It also helps consolidate which functionalities are expcted to be
exposed outside of the application via API. There could be a variety of exported
functions in the domain packages, which are not meant to communicate with
anything outside the application rather to be used among other domain packages.

But remember, middleware handling is still at the internal/server layer. e.g.
access log, authentication etc. Even though this can be brought to the `api`
package, it doesn't make much sense because middleware are mostly dependent on
the server/handler implementation. e.g. HTTP method, path etc.

<p align="right">(<a href="#readme-top">back to top</a>)</p>

## internal/users

Users package is where all your actual user related _business logic_ is
implemented. e.g. Create a user after cleaning up the input, validation, and
then store it inside a persistent datastore.

The `store_postgres.go` in this package is where you write all the direct
interactions with the datastore. There's an interface which is unique to the
`users` package. It is used to handle dependency injection as well as dependency
inversion elegantly. The file naming convention I follow is to have the word
`store` in the beggining, suffixed with `_<db name>`. Though I think it's also
ok name it based on a logical group, e.g. `store_registration`, `store_login`
etc. Especially when there's a lot of database/storage related to code to be
crammed into a single file.

`NewService/New` function is created in each package, which initializes and
returns the respective package's feature _implementor_. In case of users
package, it's the `Users` struct. The name 'NewService' makes sense in most
cases, and just reduces the burden of thinking of a good name for such
scenarios. The Users struct here holds all the dependencies required for
implementing features provided by users package.

## internal/users_test

There's quite a lot of discussions about achieveing and maintaining 100% test
coverage or not. 100% coverage sounds very nice, but might not always be
practical or at times not even possible. What I like doing is, writing unit test
for your core business logic, in this case 'Sanitize', 'Validate' etc are my
business logic.

It is important for us to understand the purpose of unit tests. The sole purpose
of unit test is unironically "test the purpose of the unit/function". It is
_*not*_ to check the implementation, how it's done, how much time it took, how
efficient it is etc. The sole purpose is to validate "what it does". This is why
you see a lot of unit tests will have hardcoded values, because those are
reliable/verified human input which we validate against.

<p align="right">(<a href="#readme-top">back to top</a>)</p>

### integration tests

In case of writing integration tests, i.e. when you make API calls from outside
the app to test functionality, I prefer using actual running instances of
dependencies instead of mocks. Especially in case of databases, or any such easy
to use dependency. Though if the dependency is an external service's APIs, mocks
are probably the best available option.

## internal/usernotes

Similar to the users package, 'usernotes' handles all business logic related to
user's notes.

## internal/pkg

`pkg` directory contains all the packages which are to be consumed across
multiple packages within the project. For instance the _*postgres*_ package will
be consumed by both users and usernotes package. It's the destination for
utility packages, and nothing to do with the features of your main application.

### internal/pkg/postgres

The postgres package initializes `pgxpool.Pool` and returns a new instance.
Though a seemingly redundant package only for initialization, it's useful to do
all the default configuration which we want standardized across the application.
An example is to wrap the driver, or functions for
[APM](https://en.wikipedia.org/wiki/Application_performance_management). The
screenshots below show how APM can help us monitor our application.

### internal/pkg/logger

I usually define the logging interface as well as the package, in a private
repository (internal to your company e.g. vcs.yourcompany.io/gopkgs/logger), and
is used across all services. Logging interface helps you to easily switch
between different logging libraries, as all your apps would be using the
interface **you** defined (interface segregation principle from SOLID). Though
here I'm making it part of the application itself as it has fewer chances of
going wrong when trying to cater to a larger audience.

### internal/pkg/jwt

The JWT package is used to generate and validate JWT tokens. It is used across
all services.

### internal/pkg/apm

The APM package is used to initialize and return a new instance of the
application performance management (APM) library. It is used across all
services.

<p align="right">(<a href="#readme-top">back to top</a>)</p>

## cmd/server/http

All HTTP related configurations and functionalities are kept inside this
package. The naming convention followed for filenames, is also straightforward.
i.e. all the HTTP handlers of a specific package/domain are grouped under
`handlers_<business logic unit name>.go`. The special mention of naming handlers
is because, often for decently large web applications (especially when building
REST-ful services) you end up with a lot of handlers. I have services with 100+
handlers, so keeping them organized helps.

## db

This directory contains database migration files (`db/migrations`). Instead of
relying on `init.sql` inside the application or database container, we use
versioned migrations.

This approach ensures that database schema changes are tracked, reproducible,
and can be applied safely across different environments (dev, staging,
production). Recommended tool:
[golang-migrate](https://github.com/golang-migrate/migrate)

```bash
# Example usage with golang-migrate
$ migrate -path db/migrations -database "postgres://user:pass@localhost:5432/dbname?sslmode=disable" up
```

## docker

The `docker/` directory contains the Dockerfile and related assets for
containerization. This separation keeps the root directory clean and allows for
managing multiple Dockerfiles or build strategies if needed in the future.

The project uses a multi-stage build process to ensure small and secure
production images.

## kubernetes (k8s)

This project includes a complete Kubernetes configuration in the `k8s/`
directory. It uses a standard Deployment + Service model.

- **app.yaml**: Main application deployment including Liveness/Readiness/Startup
  probes.
- **postgres.yaml**: Database deployment with Persistent Volume claims (in
  production) or ephemeral storage.
- **secrets.yaml**: Managing sensitive data like DB passwords and JWT secrets
  (ensure this is not committed solely in production, utilize Secret Stores).
- **app-config.yaml**: ConfigMaps for non-sensitive environment variables.

> **Tip**: For a better local development experience with Kubernetes, I
> recommend using [Tilt](https://tilt.dev/). It provides fast hot reloading and
> a great UI for monitoring your services running in K8s.

## main.go

Finally the `main package`. Placing `main.go` in the root is perfectly valid.
However, if your project requires multiple entry points—for example, if you want
running the HTTP server and gRPC server as separate commands (`cmd/http` and
`cmd/grpc`)—then utilizing the `cmd/` directory becomes the preferred structure.

The responsibility of the main package is one and only one: **get things
started**.

## Error handling

Effective error handling is crucial for reliable applications. This project
leverages the [naughtygopher/errors](https://github.com/naughtygopher/errors)
package to streamline troubleshooting and API response generation.

It serves as a drop-in replacement for Go's built-in errors, allowing us to
capture full error details (stack traces, etc.) for logging while sending clean,
user-friendly messages to the API client.

> **Note**: While useful for now, future iterations of this boilerplate should
> aim to reduce dependency on external error wrapping libraries in favor of
> standard Go 1.13+ error wrapping features or a lightweight internal
> implementation.

<p align="right">(<a href="#readme-top">back to top</a>)</p>

## Dependency flow

<p align="center">
<img src="https://user-images.githubusercontent.com/1092882/104085767-f5999100-5277-11eb-808a-5fd9b6776ad6.png" alt="Dependency flow between the layers" width="768px"/>
</p>

## Integrating Open telemetry for instrumentation

[Open telemetry](https://opentelemetry.io/) released their
[first stable version,v1.23.0, in Feb 2024](https://github.com/open-telemetry/opentelemetry-go/releases/tag/v1.23.0),
and is supported by most observability providers. You can find
[Go's Open telemetry libraries here](https://opentelemetry.io/docs/instrumentation/go/).

### Configuration

APM features (Metrics & Tracing) can be easily toggled on or off using
environment variables without code changes. This is useful for turning off heavy
instrumentation in local development or specific environments.

- `ENABLE_METRICS`: Set to `false` to disable Prometheus metrics (defaults to
  `true`).
- `ENABLE_TRACING`: Set to `false` to disable OpenTelemetry tracing (defaults to
  `true`).

Example `.envrc`:

```bash
export ENABLE_METRICS=false
export ENABLE_TRACING=false
```

<!-- MARKDOWN LINKS & IMAGES -->

[actions-shield]:
  https://img.shields.io/github/actions/workflow/status/naughtygopher/goapp/go.yml?branch=master&style=for-the-badge
[actions-url]: https://github.com/baobei23/goapp/actions
[godoc-shield]:
  https://img.shields.io/badge/godoc-reference-blue?style=for-the-badge&logo=go&logoColor=white
[godoc-url]: http://godoc.org/github.com/baobei23/goapp
[license-shield]:
  https://img.shields.io/github/license/baobei23/goapp.svg?style=for-the-badge
[license-url]: https://github.com/baobei23/goapp/blob/main/LICENSE
[linkedin-shield]:
  https://img.shields.io/badge/LinkedIn-0077B5?style=for-the-badge&logo=linkedin&logoColor=white
[linkedin-url]: https://linkedin.com/in/ahmadreginald
