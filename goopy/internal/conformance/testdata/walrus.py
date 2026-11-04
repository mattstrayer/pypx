# Module level
if x := f(): pass
if (x := f()) is not None: pass


def after_module_level():
    """Still a module function."""


def in_function():
    if x := f(): pass
    if (x := f()) is not None: pass


def after_function():
    """Still a module function."""


class C:
    def m(self):
        if x := f(): pass
        if (x := f()) is not None: pass

    def after_m(self):
        """Still a method of C."""


def after_c():
    """Still a module function."""
