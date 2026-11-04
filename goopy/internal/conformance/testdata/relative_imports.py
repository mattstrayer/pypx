# Module level
from .... import a
from ....b import a
from ..... import a
from . import a
from .. import a
from ...b import c


def after_module_level():
    """Still a module function."""


def in_function():
    from .... import a
    from ....b import a
    from ..... import a
    from . import a
    from .. import a
    from ...b import c


def after_function():
    """Still a module function."""


class C:
    def m(self):
        from .... import a
        from ....b import a
        from ..... import a
        from . import a
        from .. import a
        from ...b import c

    def after_m(self):
        """Still a method of C."""


def after_c():
    """Still a module function."""
