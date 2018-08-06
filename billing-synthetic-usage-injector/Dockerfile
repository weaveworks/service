FROM alpine:3.8
EXPOSE 80
ENTRYPOINT ["/injector"]
RUN apk --no-cache add ca-certificates
ADD /billing-synthetic-usage-injector/injector /

ARG revision
LABEL maintainer="Weaveworks <help@weave.works>" \
      org.opencontainers.image.title="billing-synthetic-usage-injector" \
      org.opencontainers.image.description="The billing-synthetic-usage-injector mocks a Weave Cloud instance and generates fake usage, a.k.a. 'synthetic load'. This synthetic load is useful to test the billing pipeline end-to-end, at runtime." \
      org.opencontainers.image.source="https://github.com/weaveworks/service/tree/master/billing-synthetic-usage-injector" \
      org.opencontainers.image.revision="${revision}" \
      org.opencontainers.image.vendor="Weaveworks"
