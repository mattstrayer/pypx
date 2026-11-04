# Module level
if a == 1: pass
for a in sorted(b, key=lambda x: x): pass
while f(a or b): pass
with open(a / 2) as fh: pass


def after_module_level():
    """Still a module function."""


def in_function():
    if a == 1: pass
    for a in sorted(b, key=lambda x: x): pass
    while f(a or b): pass
    with open(a / 2) as fh: pass


def after_function():
    """Still a module function."""


class C:
    def m(self):
        if a == 1: pass
        for a in sorted(b, key=lambda x: x): pass
        while f(a or b): pass
        with open(a / 2) as fh: pass

    def after_m(self):
        """Still a method of C."""


def after_c():
    """Still a module function."""
