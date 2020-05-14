FROM golang:1

ENV PROJECT=content-rw-elasticsearch

ENV ORG_PATH="github.com/Financial-Times"
ENV SRC_FOLDER="${GOPATH}/src/${ORG_PATH}/${PROJECT}"
ENV BUILDINFO_PACKAGE="${ORG_PATH}/service-status-go/buildinfo."

ARG GITHUB_USERNAME
ARG GITHUB_TOKEN

COPY . ${SRC_FOLDER}
WORKDIR ${SRC_FOLDER}

# Install statik cli tool in GOPATH in order to successfully execute the go generate command
RUN echo "machine github.com login $GITHUB_USERNAME password $GITHUB_TOKEN" > ~/.netrc \
  && GOPRIVATE="github.com/Financial-Times" \
  && GO111MODULE=off go get -u github.com/myitcv/gobin \
  # Get statik version from go.mod of the project
  && STATIK_VERSION="$(go list -mod=readonly -m all | grep statik | cut -d ' ' -f2)" \
  && gobin github.com/rakyll/statik@${STATIK_VERSION} \
  && go generate ./cmd/${PROJECT}

# Build app
RUN VERSION="version=$(git describe --tag --always 2> /dev/null)" \
  && DATETIME="dateTime=$(date -u +%Y%m%d%H%M%S)" \
  && REPOSITORY="repository=$(git config --get remote.origin.url)" \
  && REVISION="revision=$(git rev-parse HEAD)" \
  && BUILDER="builder=$(go version)" \
  && LDFLAGS="-X '"${BUILDINFO_PACKAGE}$VERSION"' -X '"${BUILDINFO_PACKAGE}$DATETIME"' -X '"${BUILDINFO_PACKAGE}$REPOSITORY"' -X '"${BUILDINFO_PACKAGE}$REVISION"' -X '"${BUILDINFO_PACKAGE}$BUILDER"'" \
  && CGO_ENABLED=0 go build -mod=readonly -a -o /artifacts/${PROJECT}/${PROJECT} -ldflags="${LDFLAGS}" ./cmd/${PROJECT} \
  && echo "Build flags: ${LDFLAGS}"

# Multi-stage build - copy only the certs and the binary into the image
FROM scratch
WORKDIR /
COPY --from=0 /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=0 /artifacts/* /

CMD [ "/content-rw-elasticsearch" ]
