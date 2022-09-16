# Overview (Drone 0.4)

The examples below may reference GAE options that **are no longer supported by GAE**.
They are only here to provide as example workflow configurations.

## Basic example of capable of deploying a new version of a Go, PHP and Python 'hello, world' application to standard App Engine.

```yml
- name: gae
  pull: if-not-exists
  settings:
    action: update
    project: my-gae-project
    gae_credentials:
      from_secret: GOOGLE_CREDENTIALS
    version: "${DRONE_COMMIT}"
  when:
    branch:
    - main
    event:
    - push
```

## Testing, deploying and migrating traffic to a Go application.

```yml
- name: test
  pull: if-not-exists
  image: jprobinson/ae-go-buildbox:1.6
  commands:
  - goapp get -t
  - goapp test -v -cover
  when:
    event:
    - push
    - pull_request

- name: gae_new
  pull: if-not-exists
  image: nytimes/drone-gae
  environment:
    MY_SECRET_DEV:
      from_secret: SECRET_DEV
  settings:
    action: update
    ae_environment:
      MY_SECRET: $${MY_SECRET_DEV}
    project: my-gae-project
    gae_credentials:
      from_secret: GOOGLE_CREDENTIALS
    version: "${DRONE_COMMIT}"
  environment:
    GOPATH: /drone
  when:
    branch:
    - main
    event:
    - push

- name: gae_migrate
  pull: if-not-exists
  image: nytimes/drone-gae
  settings:
    action: set_default_version
    project: my-gae-project
    gae_credentials:
      from_secret: GOOGLE_CREDENTIALS
    version: "${DRONE_COMMIT}"
  when:
    branch:
    - main
    event:
    - push
```

## Deploying multiple applications from the same git repository using the 'dir' option:

```yml
- name: gae_frontend
  pull: if-not-exists
  image: nytimes/drone-gae
  settings:
    action: update
    dir: frontend
    project: my-gae-project
    gae_credentials:
      from_secret: GOOGLE_CREDENTIALS
    version: "${DRONE_COMMIT}"
  when:
    branch:
    - main
    event:
    - push

- name: gae_api
  pull: if-not-exists
  image: nytimes/drone-gae
  settings:
    action: update
    dir: api
    project: my-gae-project
    gae_credentials:
      from_secret: GOOGLE_CREDENTIALS
    version: "${DRONE_COMMIT}"
  when:
    branch:
    - main
    event:
    - push
```

## Deploying an application to dev via pushes to main and to prd via git tags:

```yml
- name: gae_dev
  pull: if-not-exists
  image: nytimes/drone-gae
  settings:
    action: update
    app_file: dev.yaml
    project: my-dev-project
    gae_credentials:
      from_secret: GOOGLE_CREDENTIALS
    version: "${DRONE_COMMIT}"
  when:
    branch:
    - main
    event:
    - push

- name: gae_prd
  pull: if-not-exists
  image: nytimes/drone-gae
  settings:
    action: update
    app_file: prd.yaml
    project: my-prd-project
    gae_credentials:
      from_secret: GOOGLE_CREDENTIALS
    version: "${DRONE_COMMIT}"
  when:
    event:
    - tag
```

## Building a Docker image, pushing it to GCR and then deploying it via `gcloud app deploy`:

```yml
- name: compile
  pull: if-not-exists
  image: jprobinson/ae-go-buildbox:1.6
  commands:
  - go test -v -race ./...
  - go build -o api .
  environment:
    GOPATH: /drone
  when:
    event:
    - push
    - pull_request

- name: gcr
  pull: if-not-exists
  settings:
    repo: my-gae-project/api
    storage_driver: overlay
    tag: "${DRONE_COMMIT}"
    gae_credentials:
      from_secret: GOOGLE_CREDENTIALS
  when:
    branch:
    - develop
    - main
    event:
    - push

- name: gae
  pull: if-not-exists
  image: nytimes/drone-gae
  settings:
    action: deploy
    addl_flags:
    - --stop-previous-version
    flex_image: gcr.io/my-gae-project/puzzles-sub:"${DRONE_COMMIT}"
    project: my-gae-project
    gae_credentials:
      from_secret: GOOGLE_CREDENTIALS
    version: "${DRONE_COMMIT:0:10}"
  when:
    branch:
    - develop
    event:
    - push
```
