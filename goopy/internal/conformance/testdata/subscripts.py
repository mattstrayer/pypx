# Module level
for x in y[:-1]: pass
for a in sorted(b, key=lambda x: x): pass
y[1, :] = 1


def after_module_level():
    """Still a module function."""


def in_function():
    for x in y[:-1]: pass
    for a in sorted(b, key=lambda x: x): pass
    y[1, :] = 1


def after_function():
    """Still a module function."""


class C:
    def m(self):
        for x in y[:-1]: pass
        for a in sorted(b, key=lambda x: x): pass
        y[1, :] = 1

    def after_m(self):
        """Still a method of C."""


def after_c():
    """Still a module function."""
