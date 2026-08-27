FROM gcr.io/google.com/cloudsdktool/cloud-sdk:328.0.0

# Debian buster packages were pulled from deb.debian.org after buster's LTS ended; use the archive instead.
RUN sed -i 's|deb.debian.org|archive.debian.org|g; s|security.debian.org|archive.debian.org|g' /etc/apt/sources.list \
    && apt-get update \
    && apt-get install -qqy unzip

ENV GOOGLE_APP_ENGINE_SDK_VERSION=1.9.68

# The appengine-sdks GCS bucket is no longer served by Google; pull the last known-good
# archive.org capture of this SDK version instead.
RUN curl -fsSLo go_appengine_sdk_linux_amd64-$GOOGLE_APP_ENGINE_SDK_VERSION.zip https://web.archive.org/web/20251011080139if_/https://storage.googleapis.com/appengine-sdks/featured/go_appengine_sdk_linux_amd64-$GOOGLE_APP_ENGINE_SDK_VERSION.zip
RUN unzip -q go_appengine_sdk_linux_amd64-$GOOGLE_APP_ENGINE_SDK_VERSION.zip
RUN rm go_appengine_sdk_linux_amd64-$GOOGLE_APP_ENGINE_SDK_VERSION.zip

ADD drone-gae /bin/
ENTRYPOINT ["/bin/drone-gae"]
