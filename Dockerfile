FROM google/cloud-sdk:224.0.0-alpine

RUN apk add -U --no-cache unzip

# Install appengine SDK + prune unnecessary goroots
RUN gcloud components install app-engine-go \
		&& rm -rf google-cloud-sdk/platform/google_appengine/goroot-1.9

ADD drone-gae /bin/
ENTRYPOINT ["/bin/drone-gae"]
