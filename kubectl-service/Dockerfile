FROM alpine:3.7
RUN apk add --no-cache -q ca-certificates

RUN mkdir kubectl
WORKDIR /kubectl

RUN apk --no-cache -q add curl && \
        curl -fsSLo 1.15.11 "http://storage.googleapis.com/kubernetes-release/release/v1.15.11/bin/linux/amd64/kubectl" && \
        echo -n "4b9053d6ffd34c68a16af1d99855e68d27b7578f75382f19648d425f29f0fbc5  1.15.11" | sha256sum -c && \
        chmod +x 1.15.11 && \
        curl -fsSLo 1.16.9 "http://storage.googleapis.com/kubernetes-release/release/v1.16.9/bin/linux/amd64/kubectl" && \
        echo -n "0f3a6618a2e7402b11a1d9b9ffeff3ba0c6765dc361815413ce7441799aecf96  1.16.9" | sha256sum -c && \
        chmod +x 1.16.9 && \
        curl -fsSLo 1.17.5 "http://storage.googleapis.com/kubernetes-release/release/v1.17.5/bin/linux/amd64/kubectl" && \
        echo -n "03cd1fa19f90d38005148793efdb17a9b58d01dedea641a8496b9cf228db3ab4  1.17.5" | sha256sum -c && \
        chmod +x 1.17.5 && \
        apk del --purge -q curl

RUN ln -s 1.17.5 latest

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
