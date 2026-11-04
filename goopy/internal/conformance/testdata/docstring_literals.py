def plain():
    "doc"


def raw():
    r"doc"


def unicode_prefix():
    u"doc"


def concatenated():
    "a" "b"


def parenthesized():
    ("doc")


def empty():
    ""


def fstring():
    f"""x"""


def fstring_upper():
    F"x"


def raw_fstring():
    rf"x"


def bytes_literal():
    b"x"


def raw_bytes():
    Rb"x"


def str_then_fstring():
    "a" f"b"


def fstring_then_str():
    f"a" "b"


class PlainClass:
    "doc"


class FstringClass:
    f"x"


class BytesClass:
    b"x"
