#!/bin/sh
if [ "$IS_CONTAINER" != "" ]; then
  yamllint -d relaxed ./examples/
else
  if command -v "podman" &> /dev/null; then
    ENGINE=podman
  elif command -v "docker" &> /dev/null; then
    ENGINE=docker
  else
    echo "No podman/docker present, please install and retry"
    exit 1
  fi

  ${ENGINE} run --rm \
    --env IS_CONTAINER=TRUE \
    --volume "${PWD}:/workdir:z" \
    --entrypoint sh \
    quay.io/coreos/yamllint \
    ./hack/yaml-lint.sh
fi;
