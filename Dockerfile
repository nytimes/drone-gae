FROM google/cloud-sdk:224.0.0-alpine

RUN apk add -U --no-cache unzip

# Install appengine SDK
RUN gcloud components install app-engine-go

# Make sure appcfg is executable
RUN chmod +x /google-cloud-sdk/platform/google_appengine/appcfg.py

ADD drone-gae /bin/
ENTRYPOINT ["/bin/drone-gae"]
