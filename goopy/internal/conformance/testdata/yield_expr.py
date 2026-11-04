# Module level
x = yield
x = yield a, b


def after_module_level():
    """Still a module function."""


def in_function():
    x = yield
    x = yield a, b


def after_function():
    """Still a module function."""


class C:
    def m(self):
        x = yield
        x = yield a, b

    def after_m(self):
        """Still a method of C."""


def after_c():
    """Still a module function."""
