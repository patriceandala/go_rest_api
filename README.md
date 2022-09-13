# storefront-http

[![Coverage Status](https://coveralls.io/repos/github/dropezy/storefront-backend/badge.svg?branch=cov-http&t=6ByAF2)](https://coveralls.io/github/dropezy/storefront-backend?branch=cov-http)

Is a http server that serves additional operations such as handling callback from 3rd party integrations.

 tream  `main` branch.

### Running the server locally

There are few tools that are required to run this server locally i.e: Go, docker, and docker-compose.

If you already have this installed you can run `make server`, this command will build the server inside a container and run it via docker-compose.

Or you can run `go run .` and access `http://localhost:8443`

To test, run `make install` and then `make test` to test and generate the coverage

## Handlers

TODO: register available handlers here (for the start we only need to register callback handlers)
