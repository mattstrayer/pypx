# Module level
a, b = 1, 2
self.x = int(1), int(2)
a += 1, 2
x = *a, b


def after_module_level():
    """Still a module function."""


def in_function():
    a, b = 1, 2
    self.x = int(1), int(2)
    a += 1, 2
    x = *a, b


def after_function():
    """Still a module function."""


class C:
    def m(self):
        a, b = 1, 2
        self.x = int(1), int(2)
        a += 1, 2
        x = *a, b

    def after_m(self):
        """Still a method of C."""


def after_c():
    """Still a module function."""
