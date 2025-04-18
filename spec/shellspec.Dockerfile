# Dockerfile
FROM shellspec/shellspec-debian
RUN apt-get update && apt-get install -y --no-install-recommends gawk jq curl && apt-get clean && rm -rf /var/lib/apt/lists/*
COPY ./ /src
SHELL ["/bin/bash", "-c"]
