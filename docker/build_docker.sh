#!/bin/sh
set -e

export DOCKER_BUILDKIT=1

docker image build \
	-t mctrack:latest \
	-t docker.zentria.ee/mctrack/mctrack:latest \
	-f docker/Dockerfile \
	.
