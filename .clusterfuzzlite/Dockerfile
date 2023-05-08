FROM gcr.io/oss-fuzz-base/base-builder-go:v1
RUN apt-get update && apt-get install -y make autoconf automake libtool
COPY . $SRC/agentbaker
WORKDIR agentbaker
COPY .clusterfuzzlite/build.sh $SRC/
