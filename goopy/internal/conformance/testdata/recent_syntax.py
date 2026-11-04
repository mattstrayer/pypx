"""Fixture: syntax from recent Python versions."""
lazy import json
lazy from os import path

lazy = 1
lazy(1)


def unpack(b):
    x = [*a for a in b]
    y = {*a for a in b}
    z = {**a for a in b}
    w = (*a for a in b)
    return f(*a for a in b)


try:
    pass
except ValueError, TypeError:
    def in_except(): pass

try:
    pass
except* OSError:
    def in_except_star(): pass


def after_all(): pass
