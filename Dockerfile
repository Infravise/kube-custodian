ARG SOURCE_TAG=1.23.2-alpine

FROM public.ecr.aws/docker/library/golang:${SOURCE_TAG} AS builder

COPY . $GOPATH/src/kube-custodian

WORKDIR $GOPATH/src/kube-custodian

RUN go mod tidy && go build .

FROM public.ecr.aws/docker/library/alpine:3.21.0

COPY --from=builder /go/src/kube-custodian/kube-custodian /usr/local/bin/

ENTRYPOINT ["kube-custodian"]
