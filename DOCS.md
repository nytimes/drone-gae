# Overview

## Credentials

`drone-gae` requires a Google service account and uses it's [JSON credential file][service-account] to authenticate.

The plugin expects the credential in the `GAE_CREDENTIALS` environment variable.
See the [official documentation][docs-secrets] for uploading secrets.

Creating and updating GAE applications requires specific GCP roles.
Refer to GAE [Access Control](https://cloud.google.com/appengine/docs/flexible/python/access-control#primitive_roles) definitions to find out what role(s) the Service Account should be assigned.
Use least permissible role for the tasks required.
Typically for `action: deploy` this is `App Engine Admin` and `Cloud Build Service Account`.

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
      APP_KEY: $${MY_KEY}
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
