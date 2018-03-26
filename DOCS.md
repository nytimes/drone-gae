# Overview (Drone 0.6+)

## Credentials

`drone-gae` requires a Google service account and uses it's [JSON credential file][service-account] to authenticate.

The plugin expects the credential in the `GAE_CREDENTIALS` environment variable.
See the [official documentation][docs-secrets] for uploading secrets.

Either:
- a) Name the secret `GAE_CREDENTIALS` and include it in the `secrets` block
- b) Follow "Alternate Names" in the doc, setting the `target` to `GAE_CREDENTIALS`

[docs-secrets]: http://docs.drone.io/manage-secrets/
[service-account]: https://cloud.google.com/iam/docs/service-accounts

## Expanding environment variables

It may be desired to reference an environment variable for use in the App Engine configuration files or the service's environment.
The plugin will automatically [expand the environment variable][expand] for the variables in `vars` and `ae_environment`.

For example when trying to using a secret in Drone to configure an environment variable through `vars`:

```yml
# .drone.yml
vars:
  TOKEN: $${SECRET}
secrets: [secret]
```

```yml
# app.yaml
env_variables:
  API_TOKEN: {{ .TOKEN }}
```

To use `$${SECRET}` or `$SECRET`, see the [Drone docs][environment] about preprocessing.
`${SECRET}` will be preprocessed to an empty string.

[expand]: https://golang.org/pkg/os/#ExpandEnv
[environment]: http://docs.drone.io/environment/
