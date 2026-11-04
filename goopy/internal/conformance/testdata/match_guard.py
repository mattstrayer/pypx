# Module level
match x:
    case [1, *r] if r:
        pass


def after_module_level():
    """Still a module function."""


def in_function():
    match x:
        case [1, *r] if r:
            pass


def after_function():
    """Still a module function."""


class C:
    def m(self):
        match x:
            case [1, *r] if r:
                pass

    def after_m(self):
        """Still a method of C."""


def after_c():
    """Still a module function."""
