"""Pytest fixtures and configuration for E2E testing on real OpenWrt hardware / simulator.
"""
import pytest
from typing import Dict, Any, Generator
from agent.config import Config
from agent.executor import RouterExecutor
from agent.skill_store import SkillStore


@pytest.fixture(scope="session")
def router_config() -> Config:
    """Fixture providing configuration loaded from test environment."""
    return Config.from_env()


@pytest.fixture(scope="module")
def executor(router_config: Config) -> RouterExecutor:
    """Fixture providing dry-run executor for E2E testing."""
    return RouterExecutor(
        hostname=router_config.router_host,
        username=router_config.router_user,
        dry_run=True,
    )


@pytest.fixture(scope="module")
def skill_store() -> Generator[SkillStore, None, None]:
    """Fixture providing temporary SQLite skill store for test scenarios."""
    store = SkillStore(db_path="tests/e2e_test_skills.db")
    yield store
    store.close()
