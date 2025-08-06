# global arguments
ARG TZ_ARG
ARG AUTHOR
ARG VERSION
ARG TAG
ARG BUILD_TIME
ARG RUN_NUMBER

FROM uzie17/debian:stable-Jakarta AS base
LABEL maintainer="Muhammad Febrian Ardiansyah <mfardiansyah.id@gmail.com>"
WORKDIR /app

ARG TZ_ARG

# CERT PACKAGES
RUN apt-get update
RUN apt-get install -y ca-certificates

RUN apt-get update && \
    apt-get install -yq tzdata && \
    ln -fs /usr/share/zoneinfo/Asia/Jakarta /etc/localtime && \
    dpkg-reconfigure -f noninteractive tzdata
ENV TZ=$TZ_ARG

#FROM repository.konsulin.care/repository/private/be-konsulin:latest as gobuild
FROM konsulin/rest-backend-vendor:develop as gobuild
LABEL stage=gobuild

# captures argument
ARG GIT_COMMIT
# e.g. latest, development, production
ARG VERSION=latest
ARG BUILD_TIME
ARG TAG
ARG TZ_ARG
ARG AUTHOR

ENV CGO_ENABLED=0
ENV GOOS=linux
ENV GO111MODULE=on

RUN echo "Set ARG value of [AUTHOR] as AUTHOR"
RUN echo "Set ARG value of [GIT_COMMIT] as $GIT_COMMIT"
RUN echo "Set ARG value of [VERSION] as $VERSION"
RUN echo "Set ARG value of [BUILD_TIME] as $BUILD_TIME"
RUN echo "Set ARG value of [TAG] as $TAG"

# get current commit and create build number
ARG RELEASE_NOTE="author=$AUTHOR \nversion=$VERSION \ncommit=${GIT_COMMIT} \ntag=$TAG \nbuild time=$BUILD_TIME \nrun number=$RUN_NUMBER"
RUN echo "${RELEASE_NOTE}" > /go/src/github.com/konsulin-id/be-konsulin/RELEASE

ADD . ./
ADD go.mod go.sum ./
ADD cmd ./cmd
ADD cmd/http ./cmd/http
#ADD cmd/example ./cmd/example
ADD internal ./internal
ADD pkg ./pkg

# updates vendor
RUN go mod tidy && go mod vendor

# builds
RUN go build -o api-service \
    -ldflags "-X main.Version=$VERSION -X main.Tag=$TAG" \
    /go/src/github.com/konsulin-id/be-konsulin/cmd/http
#    /go/src/github.com/konsulin-id/be-konsulin/cmd/example

FROM base AS release

COPY --from=gobuild /go/src/github.com/konsulin-id/be-konsulin/api-service .
COPY --from=gobuild /go/src/github.com/konsulin-id/be-konsulin/RELEASE ./RELEASE

ENTRYPOINT ["./api-service"]
