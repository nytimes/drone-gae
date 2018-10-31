FROM google/cloud-sdk:latest

RUN apt-get install -qqy unzip

ENV GOOGLE_APP_ENGINE_SDK_VERSION=1.9.68

# Install the legacy app engine SDK
RUN curl -fsSLo go_appengine_sdk_linux_amd64-$GOOGLE_APP_ENGINE_SDK_VERSION.zip https://storage.googleapis.com/appengine-sdks/featured/go_appengine_sdk_linux_amd64-$GOOGLE_APP_ENGINE_SDK_VERSION.zip
RUN unzip -q go_appengine_sdk_linux_amd64-$GOOGLE_APP_ENGINE_SDK_VERSION.zip
RUN rm go_appengine_sdk_linux_amd64-$GOOGLE_APP_ENGINE_SDK_VERSION.zip

ADD drone-gae /bin/
ENTRYPOINT ["/bin/drone-gae"]
