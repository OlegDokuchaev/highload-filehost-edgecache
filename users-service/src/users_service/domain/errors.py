class UserAlreadyExistsError(Exception):
    """Raised when normalized login already exists."""


class InvalidCredentialsError(Exception):
    """Raised when login/password pair is invalid."""


class PasswordPolicyError(Exception):
    """Raised when password does not satisfy policy."""


class RepositoryError(Exception):
    """Base error for repository-level DB failures."""


class UniqueConstraintViolationError(RepositoryError):
    """Raised when a DB unique constraint is violated."""
