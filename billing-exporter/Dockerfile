FROM python:3.7-alpine
LABEL maintainer="Weaveworks <help@weave.works>" \
      org.opencontainers.image.title="billing-exporter" \
      org.opencontainers.image.source="https://github.com/weaveworks/service/tree/master/billing-exporter" \
      org.opencontainers.image.vendor="Weaveworks"

RUN pip install pipenv && apk add --no-cache libpq

ADD /billing-exporter/Pipfile /billing-exporter/Pipfile.lock /
RUN apk add --no-cache postgresql-dev musl-dev gcc && pipenv install --system --deploy && apk del postgresql-dev musl-dev gcc

ADD /billing-exporter/exporter /exporter
CMD ["python", "-u", "-m", "exporter"]
EXPOSE 8000

ARG revision
LABEL org.opencontainers.image.revision="${revision}"
