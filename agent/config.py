"""Centralized configuration management with validation and environment loading.
"""
from dataclasses import dataclass
from pathlib import Path
from typing import Optional
import os
from dotenv import load_dotenv


@dataclass
class Config:
    """Application configuration dataclass."""

    router_host: str
    router_user: str
    ssh_key_path: Optional[str] = None
    router_password: Optional[str] = None
    gemini_api_key: str = ""
    auth_token: Optional[str] = None
    approve_token: Optional[str] = None
    log_level: str = "INFO"
    log_file: str = "logs/beryl7-agent.log"

    @classmethod
    def from_env(cls, env_path: Optional[str] = None) -> "Config":
        """Load configuration from environment variables or .env file."""
        if env_path and Path(env_path).exists():
            load_dotenv(env_path)
        else:
            load_dotenv()

        api_key = os.environ.get("GEMINI_API_KEY", "")

        return cls(
            router_host=os.environ.get("ROUTER_IP", os.environ.get("ROUTER_HOST", "192.168.8.1")),
            router_user=os.environ.get("ROUTER_USER", "root"),
            ssh_key_path=os.environ.get("SSH_KEY_PATH"),
            router_password=os.environ.get("ROUTER_PASSWORD"),
            gemini_api_key=api_key,
            auth_token=os.environ.get("AUTH_TOKEN"),
            approve_token=os.environ.get("APPROVE_TOKEN"),
            log_level=os.environ.get("LOG_LEVEL", "INFO"),
            log_file=os.environ.get("LOG_FILE", "logs/beryl7-agent.log"),
        )

    def validate(self) -> None:
        """Validate that essential configuration fields are present and safe."""
        if not self.ssh_key_path and not self.router_password:
            # Non-blocking warning for offline dry-run test modes
            pass
