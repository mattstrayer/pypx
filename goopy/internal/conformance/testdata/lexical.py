"""Fixture: lexical forms."""

def form_feed(): pass
def form_feed_at_start(): pass


class C:
    def m(self): pass

    def after_form_feed(self): pass


def number_then_keyword(a):
    return 1if a else 2


x = 1if form_feed else 2
y = [1for a in []]
z = 1_000e1_0 + 1.5j + 0xFF + 0o7 + 0b1 + .5 + 1. + 1e5


def café(कि, 〇=1): pass


t = t"{x}" Tr"{y}"
f = f"""{x
}""" f"{x:{y}}" f"{{}}" rf"\{x}" f"{x!r:>{y}}" f"{x['a']}" f"{'}'}"
g = f"{
    x
}"


def after_all(): pass
