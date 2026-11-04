"""Fixture: soft keywords and type parameter forms in signatures."""


def soft(type, match=1, *case, **_): pass


def star_type(*type, match): pass


def star_match(a, /, *match, **type): pass


def generic[T: int = str, *Ts = *tuple[int], **P = [int], U = int](a: T): pass


class Generic[*Ts, **P](object):
    def m[T](self, x: T) -> T: pass


type Alias[*Ts, **P, T: int = str] = int


def annotated(*args: *tuple[int], **kw: int): pass


def after_all(): pass
