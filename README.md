# drone-gae

[![Build Status](https://travis-ci.org/NYTimes/drone-gae.svg?branch=master)](https://travis-ci.org/NYTimes/drone-gae)

Drone plugin to manage deployments on Google App Engine.

## Overview

This plugin is a simple wrapper around the `appcfg.py` and `gcloud app` commands, which makes it capable of making deployments in the standard environment or flexible environments with any language available.

The `action` configuration variable (shown below) can accept any action that you would normally call on `appcfg.py` or `gcloud app`.
So far, it has been tested with `update` to deploy and `set_default_version` to migrate traffic in `appcfg` and `gcloud app deploy` for `gcloud app`, but it should also be capable of running helpful ops commands like `update_indexes` and `update_cron`.

To see a full list of configuration settings for the project, check out the [GAE struct declaration](main.go#L18-L83).

To see the App Engine SDK and `gcloud` versions, check out the [Dockerfile dependency download](Dockerfile#L3-L4).

## Drone versions compatibility

This plugin supports Drone 0.4 and 0.6+ (0.5 is deprecated).

For usage in Drone 0.5 and newer, see [these docs](DOCS.md).

For usage in Drone 0.4, see [these docs](DOCS-4.md).
