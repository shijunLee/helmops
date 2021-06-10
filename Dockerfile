# Build the manager binary
FROM golang:1.16 as builder

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download
ENV GOPROXY=https://goproxy.cn,https://goproxy.io,direct
# Copy the go source
COPY main.go main.go
COPY api/ api/
COPY pkg/ pkg/
COPY controllers/ controllers/
ARG TAG
ARG BRANCH
ARG BUILD_TIME
ARG COMMIT_ID 
ARG USERNAME
ARG PASSWORD
# Build
RUN FLAGS=`echo "-X github.com/shijunLee/helmops/pkg/version.CommitId=${COMMIT_ID} -X github.com/shijunLee/helmops/pkg/version.Branch=${BRANCH} -X github.com/shijunLee/helmops/pkg/version.Tag=${TAG} -X github.com/shijunLee/helmops/pkg/version.BuildTime=${BUILD_TIME}"` && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on go build -a -ldflags "$FLAGS" -o manager main.go

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM lishijun01/distroless:nonroot
WORKDIR /
COPY --from=builder /workspace/manager .
USER 65532:65532

ENTRYPOINT ["/manager"]
