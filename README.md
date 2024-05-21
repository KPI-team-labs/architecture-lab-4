# Balancer

## Prerequisites

- Go 1.22
- Docker

## Building the Project

1. Clone the repository
2. Build the Docker images
```shell
docker compose build
```
3. Run the Docker containers
```shell
docker compose up
```

## Running the Tests

```shell
docker-compose -f docker-compose.yaml -f docker-compose.test.yaml up --exit-code-from test
```