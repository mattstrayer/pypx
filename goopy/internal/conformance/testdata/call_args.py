# Module level
f(a == 1)
f(a or b)
x = f(a / 2)
f(a if b else c)
f(a is None for a in b)


def after_module_level():
    """Still a module function."""


def in_function():
    f(a == 1)
    f(a or b)
    x = f(a / 2)
    f(a if b else c)
    f(a is None for a in b)


def after_function():
    """Still a module function."""


class C:
    def m(self):
        f(a == 1)
        f(a or b)
        x = f(a / 2)
        f(a if b else c)
        f(a is None for a in b)

    def after_m(self):
        """Still a method of C."""


def after_c():
    """Still a module function."""
