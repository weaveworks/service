FROM python:3.6-alpine
LABEL maintainer="Weaveworks <help@weave.works>" \
      org.opencontainers.image.title="billing-exporter" \
      org.opencontainers.image.source="https://github.com/weaveworks/service/tree/master/billing-exporter" \
      org.opencontainers.image.vendor="Weaveworks"

RUN pip install pipenv

ADD Pipfile Pipfile.lock /
RUN pipenv install --system --deploy

ADD exporter /exporter
CMD ["python", "-m", "exporter"]
EXPOSE 8000

ARG revision
LABEL org.opencontainers.image.revision="${revision}"
