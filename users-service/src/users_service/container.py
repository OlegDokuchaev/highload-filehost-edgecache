from dependency_injector import containers, providers
from sqlalchemy.ext.asyncio import AsyncSession, async_sessionmaker

from users_service.config import Settings
from users_service.db.session import create_engine, create_session_factory
from users_service.services.auth import AuthService
from users_service.services.security import SecurityService


class Container(containers.DeclarativeContainer):
    wiring_config = containers.WiringConfiguration(modules=["users_service.api.routes_auth"])

    settings = providers.Singleton(Settings)

    engine = providers.Singleton(create_engine, db_url=settings.provided.db_url)
    session_factory: providers.Provider[async_sessionmaker[AsyncSession]] = (
        providers.Singleton(create_session_factory, engine=engine)
    )

    security_service = providers.Singleton(SecurityService, settings=settings)
    auth_service = providers.Factory(
        AuthService,
        session_factory=session_factory,
        security_service=security_service,
    )
