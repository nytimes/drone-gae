# Overview

## Credentials

`drone-gae` requires a Google service account and uses it's [JSON credential file][service-account] to authenticate.

The plugin expects the credential in the `GAE_CREDENTIALS` environment variable.
See the [official documentation][docs-secrets] for uploading secrets.

Creating and updating GAE applications requires specific GCP roles.
Refer to GAE [Access Control](https://cloud.google.com/appengine/docs/flexible/python/access-control#primitive_roles) definitions to find out what role(s) the Service Account should be assigned.
Use least permissible role for the tasks required.
Typically for `action: deploy` this is `App Engine Admin` and `Cloud Build Service Account`.

If the service account JSON key is provided in base64 format, the plugin will decode it internally.

For example:

```yml
# .drone.yml

# Drone 1.0+
---
kind: pipeline
# ...
steps:
  - name: deploy
    image: nytimes/drone-gae
    settings:
      # ...
    environment:
      GAE_CREDENTIALS:
        from_secret: GOOGLE_CREDENTIALS

# Drone 1.0+ alternative
---
kind: pipeline
# ...
steps:
  - name: deploy
    image: nytimes/drone-gae
    settings:
      gae_credentials:
        from_secret: GOOGLE_CREDENTIALS
      # ...

# Drone 0.8
---
pipeline:
  deploy:
    image: nytimes/drone-gae
    # ...
    secrets:
      - source: GOOGLE_CREDENTIALS
        target: GAE_CREDENTIALS
```

[docs-secrets]: http://docs.drone.io/manage-secrets/
[service-account]: https://cloud.google.com/iam/docs/service-accounts

## Templating with `vars:`

It may be desired to reference an environment variable for use in the App Engine configuration files or the service's environment.

You can pass variables to be used in Golang's templating engine when using `action: deploy` and specifying a value for `app_file:`, `cron_file:`, `dispatch_file:`, or `queue_file:`.

```yml
# .drone.yml

# Drone 1.0+
---
kind: pipeline
# ...
steps:
  - name: deploy
    image: nytimes/drone-gae
    settings:
      action: deploy
      app_file: app.yaml
      vars:
        HOST: example.com
      # ...

# Drone 0.8
---
pipeline:
  deploy:
    image: nytimes/drone-gae
    # ...
    vars:
      HOST: example.com
```

```yml
# app.yaml
env_variables:
  HOST: {{ .HOST }}
```

### Expanding environment variables

The plugin will automatically [expand the environment variable][expand] for the variables in `vars` and `ae_environment`.

For example when trying to using a secret in Drone to configure an environment variable through `vars`:

```yml
# .drone.yml

# Drone 1.0+
---
kind: pipeline
# ...
steps:
  - name: deploy
    image: nytimes/drone-gae
    settings:
      action: deploy
      app_file: app.yaml
      vars:
        API_TOKEN: $${MY_TOKEN}
        APP_KEY: $MY_KEY
      # ...
    environment:
      MY_TOKEN:
        from_secret: TOKEN
      MY_KEY:
        from_secret: KEY

# Drone 1.0+ will not work
---
kind: pipeline
# ...
steps:
  - name: deploy
    image: nytimes/drone-gae
    settings:
      action: deploy
      app_file: app.yaml
      vars:
        # this will not work, must be referenced through environment:
        API_TOKEN:
          from_secret: TOKEN
        APP_KEY:
          from_secret: KEY
      # ...

# Drone 0.8
---
pipeline:
  deploy:
    image: nytimes/drone-gae
    # ...
    action: deploy
    app_file: app.yaml
    vars:
      API_TOKEN: $${MY_TOKEN}
      APP_KEY: $MY_KEY
    secrets: [my_token, my_key]
```

```yml
# app.yaml
env_variables:
  API_TOKEN: {{ .API_TOKEN }}
  APP_KEY: {{ .APP_KEY }}
```

To use `$${XXXXX}` or `$XXXXX`, see the [Drone docs][environment] about preprocessing.
`${XXXXX}` will be preprocessed to an empty string.

[expand]: https://golang.org/pkg/os/#ExpandEnv
[environment]: http://docs.drone.io/environment/

## Usage examples

The examples below may reference GAE options that **are no longer supported by GAE**.
They are only here to provide as example workflows.

### Basic example of capable of deploying a new version of a Go, PHP and Python 'hello, world' application to standard App Engine.

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

### Testing, deploying and migrating traffic to a Go application.

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

### Deploying multiple applications from the same git repository using the 'dir' option:

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

### Deploying an application to dev via pushes to main and to prd via git tags:

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

### Building a Docker image, pushing it to GCR and then deploying it via `gcloud app deploy`:

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
