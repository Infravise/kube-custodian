ARG SOURCE_TAG=1.23.2-alpine

FROM public.ecr.aws/docker/library/golang:${SOURCE_TAG}

WORKDIR /app

ADD main.go go.mod go.sum /app/

RUN go mod tidy && go build .

ENTRYPOINT ["./kube-custodian"]
