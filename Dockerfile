FROM golang:1

ENV PROJECT=content-rw-elasticsearch

ENV ORG_PATH="github.com/Financial-Times"
ENV SRC_FOLDER="${GOPATH}/src/${ORG_PATH}/${PROJECT}"
ENV BUILDINFO_PACKAGE="${ORG_PATH}/service-status-go/buildinfo."

COPY . ${SRC_FOLDER}
WORKDIR ${SRC_FOLDER}

# Build app
RUN VERSION="version=$(git describe --tag --always 2> /dev/null)" \
  && DATETIME="dateTime=$(date -u +%Y%m%d%H%M%S)" \
  && REPOSITORY="repository=$(git config --get remote.origin.url)" \
  && REVISION="revision=$(git rev-parse HEAD)" \
  && BUILDER="builder=$(go version)" \
  && LDFLAGS="-X '"${BUILDINFO_PACKAGE}$VERSION"' -X '"${BUILDINFO_PACKAGE}$DATETIME"' -X '"${BUILDINFO_PACKAGE}$REPOSITORY"' -X '"${BUILDINFO_PACKAGE}$REVISION"' -X '"${BUILDINFO_PACKAGE}$BUILDER"'" \
  && go generate ./cmd/${PROJECT} \
  && CGO_ENABLED=0 go build -mod=readonly -a -o /artifacts/${PROJECT}/${PROJECT} -ldflags="${LDFLAGS}" ./cmd/${PROJECT} \
  && echo "Build flags: ${LDFLAGS}"

# Multi-stage build - copy only the certs and the binary into the image
FROM scratch
WORKDIR /
COPY --from=0 /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=0 /artifacts/* /

CMD [ "/content-rw-elasticsearch" ]
