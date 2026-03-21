from dependency_injector.wiring import Provide, inject
from fastapi import APIRouter, Depends, HTTPException, status

from users_service.api.schemas import (
    ErrorResponse,
    LoginRequest,
    LoginResponse,
    RegisterRequest,
    RegisterResponse,
    VerifyRequest,
    VerifyResponse,
)
from users_service.container import Container
from users_service.domain.errors import InvalidCredentialsError, UserAlreadyExistsError
from users_service.services.auth import AuthService

router = APIRouter(prefix="/auth", tags=["auth"])


@router.post(
    "/register",
    response_model=RegisterResponse,
    status_code=status.HTTP_201_CREATED,
    summary="Register new user",
    description=(
        "Creates a new user. Password must follow policy: 12-128 chars, "
        "uppercase, lowercase, digit, special character."
    ),
    responses={
        status.HTTP_400_BAD_REQUEST: {
            "model": ErrorResponse,
            "description": "Password policy violation or invalid input.",
        },
        status.HTTP_409_CONFLICT: {
            "model": ErrorResponse,
            "description": "normalized_login already exists.",
        },
    },
)
@inject
async def register(
    payload: RegisterRequest,
    auth_service: AuthService = Depends(Provide[Container.auth_service]),
) -> RegisterResponse:
    try:
        user_id, login, normalized_login = await auth_service.register(
            login=payload.login,
            password=payload.password,
        )
    except UserAlreadyExistsError as exc:
        raise HTTPException(
            status_code=status.HTTP_409_CONFLICT,
            detail=str(exc),
        ) from exc

    return RegisterResponse(
        user_id=user_id,
        login=login,
        normalized_login=normalized_login,
    )


@router.post("/login", response_model=LoginResponse)
@inject
async def login(
    payload: LoginRequest,
    auth_service: AuthService = Depends(Provide[Container.auth_service]),
) -> LoginResponse:
    try:
        access_token, user_id, normalized_login = await auth_service.login(
            login=payload.login,
            password=payload.password,
        )
    except InvalidCredentialsError as exc:
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="invalid credentials",
        ) from exc

    return LoginResponse(
        access_token=access_token,
        user_id=user_id,
        normalized_login=normalized_login,
    )


@router.post("/verify", response_model=VerifyResponse)
@inject
async def verify(
    payload: VerifyRequest,
    auth_service: AuthService = Depends(Provide[Container.auth_service]),
) -> VerifyResponse:
    active, user_id, normalized_login = auth_service.verify(payload.token)
    if not active:
        return VerifyResponse(active=False)
    return VerifyResponse(active=True, user_id=user_id, normalized_login=normalized_login)
