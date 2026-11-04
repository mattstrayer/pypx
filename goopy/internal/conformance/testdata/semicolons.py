# Module level
if a: b = 1; c = 2


def after_module_level():
    """Still a module function."""


def in_function():
    if a: b = 1; c = 2


def after_function():
    """Still a module function."""


class C:
    def m(self):
        if a: b = 1; c = 2

    def after_m(self):
        """Still a method of C."""


def after_c():
    """Still a module function."""
