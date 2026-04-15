"""Sample module for benchmarking goopy extraction.

This module contains representative Python patterns found in real-world
packages: functions with type annotations, classes with inheritance,
decorators, docstrings in multiple styles, type aliases, and various
expression forms.
"""

from __future__ import annotations

import os
import sys
from collections.abc import Callable, Iterable, Sequence
from typing import Any, Optional, TypeVar, Union, overload

T = TypeVar("T")
type Callback[T] = Callable[[T], None]

__all__ = [
    "Config",
    "Result",
    "process",
    "transform",
    "ValidationError",
]


class ValidationError(ValueError):
    """Raised when validation fails.

    Args:
        message: Human-readable error description.
        field: The field that failed validation.
        code: Machine-readable error code.
    """

    def __init__(
        self,
        message: str,
        *,
        field: str | None = None,
        code: str = "validation_error",
    ) -> None:
        super().__init__(message)
        self.field = field
        self.code = code

    def __str__(self) -> str:
        parts = [super().__str__()]
        if self.field:
            parts.append(f"field={self.field!r}")
        return ", ".join(parts)


class Config:
    """Application configuration.

    Supports loading from environment variables and dictionary merging.

    Attributes:
        debug: Enable debug mode.
        log_level: Logging level (DEBUG, INFO, WARNING, ERROR).
        max_retries: Maximum number of retry attempts.
        timeout: Request timeout in seconds.
    """

    debug: bool
    log_level: str
    max_retries: int
    timeout: float

    def __init__(
        self,
        debug: bool = False,
        log_level: str = "INFO",
        max_retries: int = 3,
        timeout: float = 30.0,
    ) -> None:
        self.debug = debug
        self.log_level = log_level
        self.max_retries = max_retries
        self.timeout = timeout

    @classmethod
    def from_env(cls) -> Config:
        """Create configuration from environment variables.

        Returns:
            Config: A new Config instance populated from env vars.

        Raises:
            ValidationError: If an environment variable has an invalid value.
        """
        return cls(
            debug=os.getenv("DEBUG", "false").lower() == "true",
            log_level=os.getenv("LOG_LEVEL", "INFO"),
            max_retries=int(os.getenv("MAX_RETRIES", "3")),
            timeout=float(os.getenv("TIMEOUT", "30.0")),
        )

    def merge(self, other: dict[str, Any]) -> Config:
        """Merge a dictionary of overrides into this config.

        Args:
            other: Dictionary of config keys to override.

        Returns:
            Config: A new Config with merged values.
        """
        kwargs = {
            "debug": other.get("debug", self.debug),
            "log_level": other.get("log_level", self.log_level),
            "max_retries": other.get("max_retries", self.max_retries),
            "timeout": other.get("timeout", self.timeout),
        }
        return Config(**kwargs)


class Result(tuple[bool, str | None]):
    """An immutable result type wrapping (success, error_message).

    Examples:
        >>> Result.ok()
        Result(True, None)
        >>> Result.fail("something went wrong")
        Result(False, 'something went wrong')
    """

    @staticmethod
    def ok() -> Result:
        """Create a successful result."""
        return Result((True, None))

    @staticmethod
    def fail(message: str) -> Result:
        """Create a failed result with an error message."""
        return Result((False, message))

    @property
    def success(self) -> bool:
        """Whether the result represents success."""
        return self[0]

    @property
    def error(self) -> str | None:
        """The error message, or None if successful."""
        return self[1]


@overload
def process(data: str) -> str: ...
@overload
def process(data: bytes) -> bytes: ...

def process(data: str | bytes) -> str | bytes:
    """Process input data, preserving its type.

    This function applies a series of transformations to the input,
    returning the same type as the input.

    Args:
        data: Input data to process. Can be str or bytes.

    Returns:
        The processed data in the same type as the input.

    Raises:
        ValidationError: If the data is empty.
        TypeError: If data is neither str nor bytes.
    """
    if not data:
        raise ValidationError("data cannot be empty", field="data")
    if isinstance(data, str):
        return data.strip().lower()
    if isinstance(data, bytes):
        return data.strip().lower()
    raise TypeError(f"expected str or bytes, got {type(data).__name__}")


def transform(
    items: Iterable[T],
    func: Callable[[T], T],
    *,
    filter_none: bool = True,
    max_items: int | None = None,
) -> list[T]:
    """Apply a transformation function to each item in the iterable.

    Args:
        items: Input iterable of items.
        func: Transformation function applied to each item.
        filter_none: If True, filter out None results.
        max_items: Maximum number of items to process.

    Returns:
        list: Transformed items.

    Examples:
        >>> transform([1, 2, 3], lambda x: x * 2)
        [2, 4, 6]
    """
    result = []
    for i, item in enumerate(items):
        if max_items is not None and i >= max_items:
            break
        transformed = func(item)
        if filter_none and transformed is None:
            continue
        result.append(transformed)
    return result


def _internal_helper(x: int) -> int:
    """This is private and should not be extracted."""
    return x + 1


if sys.version_info >= (3, 12):
    type NumberLike = int | float | complex
