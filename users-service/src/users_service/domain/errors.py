class UserAlreadyExistsError(Exception):
    """Raised when normalized login already exists."""


class InvalidCredentialsError(Exception):
    """Raised when login/password pair is invalid."""


class PasswordPolicyError(Exception):
    """Raised when password does not satisfy policy."""
