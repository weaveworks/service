FROM ubuntu
MAINTAINER Weaveworks Inc <help@weave.works>
RUN apt-get update && \
  apt-get install -y nginx nginx-extras && \
  rm -rf /var/lib/apt/lists/*
RUN rm /etc/nginx/sites-available/default && \
  ln -sf /dev/stdout /var/log/nginx/access.log && \
  ln -sf /dev/stderr /var/log/nginx/error.log
COPY prometheus.lua /lua/
COPY default.conf /etc/nginx/conf.d/
COPY routes.conf /etc/nginx/
COPY pki/scope.weave.works.crt /tls/scope.weave.works.crt
COPY pki/cloud.weave.works.crt /tls/cloud.weave.works.crt
COPY dhparam.pem /tls/dhparam.pem
EXPOSE 80 443
CMD ["nginx", "-g", "daemon off;"]
