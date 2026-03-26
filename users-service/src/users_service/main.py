from collections.abc import AsyncGenerator, Awaitable, Callable, Sequence
from contextlib import asynccontextmanager
from typing import Any, cast

from fastapi import FastAPI, Request, status
from fastapi.exceptions import RequestValidationError
from fastapi.responses import JSONResponse
from starlette.responses import Response

from users_service.api.routes_auth import router as auth_router
from users_service.container import Container
from users_service.domain.errors import PasswordPolicyError


def format_validation_error_detail(errors: Sequence[object]) -> str:
    """Текст для API из списка ошибок Pydantic; устойчиво к разным формам элементов, не бросает."""
    try:
        if not errors:
            return "invalid input"
        parts: list[str] = []
        for raw in errors:
            try:
                if isinstance(raw, dict):
                    msg = raw.get("msg") or raw.get("message")
                    if msg is not None and str(msg).strip():
                        parts.append(str(msg))
                    else:
                        parts.append(str(raw))
                else:
                    parts.append(str(raw))
            except Exception:
                parts.append("invalid input")
        return "; ".join(parts) if parts else "invalid input"
    except Exception:
        return "invalid input"


@asynccontextmanager
async def lifespan(app: FastAPI) -> AsyncGenerator[None, Any]:
    yield
    container: Container = app.state.container
    await container.engine().dispose()


async def password_policy_exception_handler(
    _request: Request,
    exc: PasswordPolicyError,
) -> JSONResponse:
    """Гарантирует 400 при PasswordPolicyError, даже если исключение не перехвачено в роуте."""
    return JSONResponse(
        status_code=status.HTTP_400_BAD_REQUEST,
        content={"detail": str(exc)},
    )


def create_app() -> FastAPI:
    container = Container()
    app = FastAPI(
        title="Users service",
        version="0.1.0",
        lifespan=lifespan,
    )
    app.state.container = container
    app.include_router(auth_router)

    # Starlette типизирует обработчик как (Request, Exception); см. mypy arg-type.
    _exc_handler: Callable[[Request, Exception], Awaitable[Response]] = cast(
        Callable[[Request, Exception], Awaitable[Response]],
        password_policy_exception_handler,
    )
    app.add_exception_handler(PasswordPolicyError, _exc_handler)

    @app.exception_handler(RequestValidationError)
    async def validation_exception_handler(
        _request: Request,
        exc: RequestValidationError,
    ) -> JSONResponse:
        try:
            detail = format_validation_error_detail(exc.errors())
            return JSONResponse(
                status_code=status.HTTP_400_BAD_REQUEST,
                content={"detail": detail},
            )
        except Exception:
            return JSONResponse(
                status_code=status.HTTP_400_BAD_REQUEST,
                content={"detail": "invalid input"},
            )

    @app.get("/healthz")
    async def healthz() -> dict[str, str]:
        return {"status": "ok"}

    return app


app = create_app()
