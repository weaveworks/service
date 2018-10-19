import logging
from datetime import timedelta
from urllib.parse import urlparse

import click


_LOG = logging.getLogger(__name__)


def daterange(start_date, end_date):
    for n in range(int ((end_date - start_date).days)):
        yield start_date + timedelta(n)


def datetime_floor_date(dt):
    return dt.replace(hour=0, minute=0, second=0, microsecond=0)


def datetime_ceil_date(dt):
    f = datetime_floor_date(dt)
    if f == dt:
        return dt
    else:
        return f + timedelta(days=1)


class UriParamType(click.ParamType):
    name = 'uri'

    def convert(self, value, param, ctx):
        try:
            return urlparse(value)
        except ValueError as e:
            self.fail(f'{value!r} is not a valid URL: {e}', param, ctx)


def inject_password_from_file(name, uri, password_file):
    if not password_file:
        return uri

    if uri.password:
        _LOG.warn('Password for %s specified in both URI and password file', name)
    with password_file as fh:
        password = fh.read()

    netloc = ''
    if uri.username:
        netloc += uri.username
    netloc += f':{password}@{uri.hostname}'
    if uri.port:
        netloc += f':{uri.port}'
    return uri._replace(netloc=netloc)


def all_equal(coll):
    it = iter(coll)
    first = next(it)
    while True:
        try:
            if first != next(it):
                return False
        except StopIteration:
            break
    return True
