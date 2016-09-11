# drone-gae

### Manage deployments on Google App Engine via drone

This plugin is a simple wrapper around the `appcfg.py` command, which makes it capable of making deployments with Go, PHP or Python projects.

The `action` configruation variable (shown below) can accept any action that you would normally call on `appcfg.py`. So far, it has been tested with `update` to deploy and `set_default_version` to migrate traffic, but it should also be capable of running helpful ops commands like `update_indexes` and `update_cron`.

This is currently very new and unstable. Please don't use it for anything important quit yet!


## Examples

### Basic example of capable of deploying a new version of a Go, PHP and Python 'hello, world' application to App Engine.

	deploy:
	  gae:
        action: update
        project: my-gae-project
	    version: "$$COMMIT"
	    token: >
	      $$GOOGLE_CREDENTIALS
	    when:
	      event: push
	      branch: master

### Testing, deploying and migrating traffic to a Go application.

	build:
	  image: jprobinson/ae-go-buildbox:1.6
	  commands:
	    - goapp get -t
	    - goapp test -v -cover
	  when:
	    event:
	      - push
	      - pull_request

	deploy:

      # deploy new version to App Engine
	  gae:
        environment:
          - GOPATH=/drone
        action: update
        project: my-gae-project
	    version: "$$COMMIT"
        ae_environment:
          MY_SECRET: $$MY_SECRET_DEV
	    token: >
	      $$GOOGLE_CREDENTIALS
	    when:
	      event: push
	      branch: master

      # set new version to 'default', which migrates 100% traffic.
	  gae:
        action: set_default_version
        project: my-gae-project
	    version: "$$COMMIT"
	    token: >
	      $$GOOGLE_CREDENTIALS
	    when:
	      event: push
	      branch: master


### Deploying multiple applications from the same git repository using the 'dir' option:

	deploy:

      # deploy new version of the 'frontend' service to App Engine
	  gae:
        action: update
        project: my-gae-project
	    version: "$$COMMIT"
        dir: frontend
	    token: >
	      $$GOOGLE_CREDENTIALS
	    when:
	      event: push
	      branch: master

      # deploy new version of the 'api' service to App Engine
	  gae:
        action: update
        project: my-gae-project
	    version: "$$COMMIT"
        dir: api
	    token: >
	      $$GOOGLE_CREDENTIALS
	    when:
	      event: push
	      branch: master


### Deploying an application to dev via pushes to master and to prd via git tags:

	deploy:

      # deploy new version of the service to App Engine on every commit to master
      # to a 'dev' project using a specific yaml file.
	  gae:
        action: update
        project: my-dev-project
	    version: "$$COMMIT"
        app_file: dev.yaml
	    token: >
	      $$GOOGLE_CREDENTIALS_DEV
	    when:
	      event: push
	      branch: master

      # deploy new version of the service to App Engine on every git tag
      # to a 'prd' project using a specific yaml file.
	  gae:
        action: update
        project: my-prd-project
	    version: "$$COMMIT"
        app_file: prd.yaml
	    token: >
	      $$GOOGLE_CREDENTIALS_PRD
	    when:
	      event: tag


## License

MIT.
