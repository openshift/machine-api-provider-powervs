FROM registry.ci.openshift.org/openshift/release:golang-1.16 AS builder
WORKDIR /go/src/github.com/openshift/cluster-api-provider-powervs
COPY . .
# VERSION env gets set in the openshift/release image and refers to the golang version, which interfers with our own
RUN unset VERSION \
 && GOPROXY=off NO_DOCKER=1 GOARCH=ppc64le make build

FROM --platform=ppc64le registry.access.redhat.com/ubi8/ubi:8.4
COPY --from=builder /go/src/github.com/openshift/cluster-api-provider-powervs/bin/machine-controller-manager /
