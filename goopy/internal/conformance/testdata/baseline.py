"""Syntax the parser already handles."""
from __future__ import annotations

import functools
from typing import TYPE_CHECKING

if TYPE_CHECKING:
    from collections.abc import Iterator

    def checking_only(x: int) -> int:
        """Defined under TYPE_CHECKING."""
        return x


def plain(a, b=1, *args, c, d=2, **kwargs):
    """Plain parameters."""
    return a


def posonly(a, b, /, c, *, d):
    return a


async def coro(x: int) -> int:
    """Async function."""
    return x


@functools.cache
def decorated(x):
    return x


class Outer:
    """Outer class."""

    class Inner:
        def deep(self):
            """Nested class method."""

    def method(self, x):
        try:
            y = x
        except ValueError:
            y = 0
        return y

    @property
    def prop(self):
        return 1

    @staticmethod
    def static(a, b):
        return a


try:
    import json
except ImportError:
    def fallback():
        """Defined in an except branch."""

else:
    def on_success():
        pass

finally:
    def always():
        pass


with open("f") as fh:
    def in_with():
        pass


def after_all():
    """Still a module function."""
