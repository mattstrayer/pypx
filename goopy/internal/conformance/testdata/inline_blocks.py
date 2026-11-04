"""Fixture: simple statements in one-line blocks."""
import os


def returns(a):
    if a: return 1
    else: return 2


def raises(a):
    if a: raise ValueError(a) from a
    for b in a: yield b
    with a: del a
    try: import json
    except ImportError: json = None
    else: from os import path
    finally: assert a, "message"


class Inline: x = 1; y = 2


if os: import sys
else: from os import sep


for i in range(3): pass
else: from os import linesep

while os: break
else: import abc


def after_all(): global G; G = 1
