FROM alpine:3.7
RUN apk add --no-cache -q ca-certificates

RUN mkdir kubectl
WORKDIR /kubectl

RUN apk --no-cache -q add curl && \
        curl -fsSLo 1.5.8 "http://storage.googleapis.com/kubernetes-release/release/v1.5.8/bin/linux/amd64/kubectl" && \
        echo -n "647e233fe0b935300a981b61245b29c7dae6af772dc1f2243cfa1970d2e90219  1.5.8" | sha256sum -c && \
        chmod +x 1.5.8 && \
        apk del --purge -q curl

RUN apk --no-cache -q add curl && \
        curl -fsSLo 1.6.13 "http://storage.googleapis.com/kubernetes-release/release/v1.6.13/bin/linux/amd64/kubectl" && \
        echo -n "17e29707dcdaac878178d4b137c798cb37993a8a7f0ae214835af4f8e322bafa  1.6.13" | sha256sum -c && \
        chmod +x 1.6.13 && \
        apk del --purge -q curl

RUN apk --no-cache -q add curl && \
        curl -fsSLo 1.7.12 "http://storage.googleapis.com/kubernetes-release/release/v1.7.12/bin/linux/amd64/kubectl" && \
        echo -n "c073997447db252a2358dc25698d8d82e2bab12b02ec54a9e8af62acab1043df  1.7.12" | sha256sum -c && \
        chmod +x 1.7.12 && \
        apk del --purge -q curl

RUN apk --no-cache -q add curl && \
        curl -fsSLo 1.8.6 "http://storage.googleapis.com/kubernetes-release/release/v1.8.6/bin/linux/amd64/kubectl" && \
        echo -n "2343a549fdfc2d7e6b43da5d7ed5be4e43abc0e7322fdbc3bdd5c0dbd24bc6a6  1.8.6" | sha256sum -c && \
        chmod +x 1.8.6 && \
        apk del --purge -q curl

RUN apk --no-cache -q add curl && \
        curl -fsSLo 1.9.1 "http://storage.googleapis.com/kubernetes-release/release/v1.9.1/bin/linux/amd64/kubectl" && \
        echo -n "ec1abbc1b91fb23e00558c09892d053012c7581afe6e5a36cada3713b0b7c37b  1.9.1" | sha256sum -c && \
        chmod +x 1.9.1 && \
        apk del --purge -q curl

RUN ln -s 1.9.1 latest

WORKDIR /
COPY kubectl-service /
EXPOSE 80
EXPOSE 4772
ENTRYPOINT ["/kubectl-service"]

ARG revision
LABEL maintainer="Weaveworks <help@weave.works>" \
      org.opencontainers.image.title="kubectl-service" \
      org.opencontainers.image.description="multi-version kubectl-as-a-service" \
      org.opencontainers.image.source="https://github.com/weaveworks/service/tree/master/kubectl-service" \
      org.opencontainers.image.revision="${revision}" \
      org.opencontainers.image.vendor="Weaveworks"
