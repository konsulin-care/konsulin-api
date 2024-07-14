#!/bin/bash
# the usage text
usage="$(basename "$0") [-h] [-a <author> -e <email> -v <version>] -- sebuah tool sederhana untuk build api service image

where:
    -h  membuka informasi pertolongan untuk cara penggunaan script ini
    -a  nama author build script ini; contoh: muhammad febrian ardiansyah
    -e  email dari author yang build script ini
    -v  version dari build (develop, staging, or production)

contoh:
    $ bash build.sh -a ardi -e mfardiansyah.id@gmail.com -v develop
    $ bash build.sh -a 'Muhammad_Febrian_Ardiansyah' -e mfardiansyah.id@gmail.com -v develop
"

# the error information when found a missing tag (GIT)
tagExample="
format:
    $ git tag -a <versi-tag> -m \"informasi tag disini\"

contoh:
    $ git tag -a v0.0.1 -m \"versi awal\"
"

# read the input arguments
while getopts ':ha:e:v:' option; do
  case "$option" in
  h)
    echo "$usage"
    exit
    ;;
  a)
    author=$OPTARG
    ;;
  e)
    email=$OPTARG
    ;;
  v)
    version=$OPTARG
    ;;
  :)
    printf "ERROR: argument -%s tidak ditemukan\n" "$OPTARG" >&2
    echo "$usage" >&2
    exit 1
    ;;
  \?)
    printf "ERROR: ditemukan argument ilegal: -%s\n" "$OPTARG" >&2
    echo "$usage" >&2
    exit 1
    ;;
  esac
done
shift $((OPTIND - 1))

if [ -z "${author}" ]; then
  printf "ERROR: value untuk argument -a (author) tidak boleh kosong\n\n"
  echo "$usage" # exit due to an error
fi

if [ -z "${email}" ]; then
  # sets default value
  printf "value untuk argument -e (email) kosong; "
  email=mfardiansyah.id@gmail.com
  echo "set default value as [${email}]"
fi

if [ -z "${version}" ]; then
  printf "value untuk argument -v (version) kosong; "
  version=develop
  echo "set default value as [${version}]"
fi

# build date
build_date=$(date '+%Y-%m-%d_%H:%M:%S_%z')

# extract a compact version of git commit (first 7 chars)
COMMIT_FULL=$(git rev-parse HEAD)
COMMIT=${COMMIT_FULL:0:7}

TAG=$(git describe --abbrev=0 --tags)
DOCKER_TAG="${TAG:1}"
echo "INFORMASI PERUBAHAN:"
echo "version = '${version}'"
echo "Git Tag terakhir = '${TAG}'"
echo "DOCKER TAG = '${DOCKER_TAG}'"
echo "COMMIT terakhir = '${COMMIT}'"
echo "Penulis pada script ini = '${author}' (${email})"
echo "Tanggal & waktu script dibuat = '${build_date}'"

if [ -z "${TAG}" ]; then
  printf "\nERROR: TAG pada GIT tidak ditemukan. Silahkan membuat tag dahulu\n"
  echo "$tagExample"
  exit 1 # exit due to an error
fi

# builds the docker image
docker build -t "konsulin/api-service:${DOCKER_TAG}" \
  --build-arg GIT_COMMIT="${COMMIT}" \
  --build-arg "AUTHOR=${author}" \
  --build-arg "VERSION=${version}" \
  --build-arg "BUILD_TIME=${build_date}" \
  --build-arg "TAG=${TAG}" .

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
