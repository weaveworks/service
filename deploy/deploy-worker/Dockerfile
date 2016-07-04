FROM debian:stretch
MAINTAINER Weaveworks Inc <help@weave.works>
WORKDIR /
RUN apt-get update && apt-get install -y git curl && rm -rf /var/lib/apt/lists/*
RUN curl -o /bin/kubectl https://storage.googleapis.com/kubernetes-release/release/v1.2.4/bin/linux/amd64/kubectl && \
    chmod u+x /bin/kubectl
COPY deploy-worker /
COPY deploy.sh kubeimage /bin/
EXPOSE 80
ENTRYPOINT ["/deploy-worker"]
