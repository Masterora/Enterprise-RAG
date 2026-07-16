from fastapi import APIRouter

from enterprise_agent.api.routers import health, runs

api_router = APIRouter()
api_router.include_router(health.router)
api_router.include_router(runs.router)
