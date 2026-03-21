from collections.abc import AsyncGenerator
from contextlib import asynccontextmanager
from typing import Any

from fastapi import FastAPI, Request, status
from fastapi.exceptions import RequestValidationError
from fastapi.responses import JSONResponse

from users_service.api.routes_auth import router as auth_router
from users_service.container import Container
from users_service.db.init_db import create_tables


@asynccontextmanager
async def lifespan(app: FastAPI) -> AsyncGenerator[None, Any]:
    container: Container = app.state.container
    await create_tables(container.engine())
    yield
    await container.engine().dispose()


def create_app() -> FastAPI:
    container = Container()
    app = FastAPI(
        title="Users service",
        version="0.1.0",
        lifespan=lifespan,
    )
    app.state.container = container
    app.include_router(auth_router)

    @app.exception_handler(RequestValidationError)
    async def validation_exception_handler(
        request: Request,
        exc: RequestValidationError,
    ) -> JSONResponse:
        # Normalize schema validation failures to API contract style.
        errors = exc.errors()
        detail = errors[0]["msg"] if errors else "invalid input"
        return JSONResponse(
            status_code=status.HTTP_400_BAD_REQUEST,
            content={"detail": detail},
        )

    @app.get("/healthz")
    async def healthz() -> dict[str, str]:
        return {"status": "ok"}

    return app


app = create_app()
