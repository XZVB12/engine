#############################################
## [golang builder]  ########################
#############################################
FROM golang:1.10 as builder

ARG VERSION
ARG GITCOMMIT

COPY . /go/src/github.com/maliceio/engine
WORKDIR /go/src/github.com/maliceio/engine/

RUN apt-get update && apt-get install -y libgit2-dev \
    && go get -d github.com/libgit2/git2go

RUN hack/build/binary

#############################################
## [malice image] ###########################
#############################################
FROM alpine:3.7

LABEL maintainer "https://github.com/blacktop"

ENV MALICE_STORAGE_PATH /malice
ENV MALICE_IN_DOCKER true

RUN apk --no-cache add ca-certificates libgit2

COPY --from=builder /go/src/github.com/maliceio/engine/cmd/malice/build/maliced /bin/maliced
WORKDIR /malice/samples

VOLUME ["/malice/config"]
VOLUME ["/malice/samples"]

EXPOSE 80 443

ENTRYPOINT ["maliced"]
CMD ["--help"]

#############################################
#############################################
#############################################
