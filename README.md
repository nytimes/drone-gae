# drone-gae

### Manage deployments on Google App Engine via drone

This plugin is a simple wrapper around the `appcfg.py` and `gcloud app` commands, which makes it capable of making deployments with Go, PHP or Python projects in the standard enviroment or any language in the flexible environment.

The `action` configruation variable (shown below) can accept any action that you would normally call on `appcfg.py` or `gcloud app`. So far, it has been tested with `update` to deploy and `set_default_version` to migrate traffic in `appcfg` and `gcloud app deploy` for `gcloud app`, but it should also be capable of running helpful ops commands like `update_indexes` and `update_cron`.

To see a full list of configuration settings for the project, check out the [GAE struct declaration](https://github.com/NYTimes/drone-gae/blob/master/main.go#L17-L64).

This is currently using `gcloud` version `135.0.0` and App Engine SDK version `1.9.46`.

## Examples

### Basic example of capable of deploying a new version of a Go, PHP and Python 'hello, world' application to standard App Engine.

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
        image: nytimes/drone-gae
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
        image: nytimes/drone-gae
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
        image: nytimes/drone-gae
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
        image: nytimes/drone-gae
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
        image: nytimes/drone-gae
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
        image: nytimes/drone-gae
        action: update
        project: my-prd-project
	    version: "$$COMMIT"
        app_file: prd.yaml
	    token: >
	      $$GOOGLE_CREDENTIALS_PRD
	    when:
	      event: tag

### Building a Docker image, pushing it to GCR and then deploying it via `gcloud app deploy`:

	build:
	  image: jprobinson/ae-go-buildbox:1.6
      environment:
        - GOPATH=/drone
	  commands:
        - go test -v -race ./...
        - go build -o api .
	  when:
	    event:
	      - push
	      - pull_request

    # runs `docker build` and `docker push` to the specified GCR
    publish:
      gcr:
        repo: my-gae-project/api
        tag: "$$COMMIT"
        token: >
           $$GOOGLE_CREDENTIALS_DEV
        storage_driver: overlay
        when:
          branch: [develop, master]
          event: push
    
	deploy:

      # deploy a new version using the docker image we just published and stop any previous versions when complete.
      gae:
        image: nytimes/drone-gae
        action: deploy
        project: my-gae-project
        flex_image: gcr.io/my-gae-project/puzzles-sub:$$COMMIT
        version: "$${COMMIT:0:10}"
        addl_flags:
         - --stop-previous-version
        token: >
          $$GOOGLE_CREDENTIALS_LAB
        when:
          event: push
          branch: develop
