
from datetime import timedelta


def datetime_floor_date(dt):
    return dt.replace(hour=0, minute=0, second=0, microsecond=0)


def datetime_ceil_date(dt):
    f = datetime_floor_date(dt)
    if f == dt:
        return dt
    else:
        return f + timedelta(days=1)