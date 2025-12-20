#!/bin/bash
docker build -t konsulin/golang-vendor:latest -f Dockerfile-vendor .

# finally, remove any dangling image
#   if there is no dangling image,
#   it will result with a warning message, "Error response from daemon: page not found"
#   you may ignore it
# images
action=$(docker images -f "dangling=true" -q)
docker rmi -f "${action}"
# volumes
action=$(docker volume ls -qf dangling=true)
docker volume rm "${action}"

exit 0
