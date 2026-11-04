"""Fixture: await in every expression position."""


async def assign(a):
    """Doc."""
    x = await a
    y: int = await a
    x += await a.b(1)
    return x, y


async def headers(a):
    if await a:
        def in_if(): pass
    elif not await a:
        def in_elif(): pass
    while await a:
        def in_while(): pass
    else:
        def in_while_else(): pass


async def nested(a):
    print(await a, k=await a, *await a, **await a)
    z = [await a, (await a), {await a: await a}, a[await a]]
    z = -await a ** 2 + await a
    z = await a if await a else await a
    return z


class After:
    """Doc."""

    async def m(self, a):
        await a; await a

    def n(self):
        pass


def after_all(): pass
