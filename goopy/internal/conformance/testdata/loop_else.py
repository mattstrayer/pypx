# Module level
for a in b:
    pass
else:
    c = 1
while a:
    pass
else:
    c = 1
async def _inner():
    async for a in b:
        pass
    else:
        c = 1


def after_module_level():
    """Still a module function."""


def in_function():
    for a in b:
        pass
    else:
        c = 1
    while a:
        pass
    else:
        c = 1
    async def _inner():
        async for a in b:
            pass
        else:
            c = 1


def after_function():
    """Still a module function."""


class C:
    def m(self):
        for a in b:
            pass
        else:
            c = 1
        while a:
            pass
        else:
            c = 1
        async def _inner():
            async for a in b:
                pass
            else:
                c = 1

    def after_m(self):
        """Still a method of C."""


def after_c():
    """Still a module function."""
