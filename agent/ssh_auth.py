"""Secure SSH authentication helper supporting key-based login and password fallback.
"""
import os
from pathlib import Path
from typing import Dict, Any, Optional
import paramiko


class SSHAuthenticator:
    """Manage SSH authentication parameters for Paramiko SSH client."""

    def __init__(self, router_host: str = "192.168.8.1", key_path: Optional[str] = None):
        self.router_host = router_host
        if key_path:
            self.ssh_key_path = Path(key_path)
        else:
            self.ssh_key_path = Path.home() / ".ssh" / "beryl7_rsa"
        self.ssh_key_passphrase = os.environ.get("SSH_KEY_PASSPHRASE")

    def get_auth_params(self) -> Dict[str, Any]:
        """Retrieve Paramiko authentication arguments.

        Priority:
        1. SSH Key file (preferred, key-based authentication)
        2. Router password fallback (if key unavailable)

        Returns:
            Dict[str, Any]: Dictionary containing key_filename or password.
        """
        if self.ssh_key_path.exists():
            params: Dict[str, Any] = {"key_filename": str(self.ssh_key_path)}
            if self.ssh_key_passphrase:
                params["password"] = self.ssh_key_passphrase
            return params

        password = os.environ.get("ROUTER_PASSWORD")
        if password:
            return {"password": password}

        return {}

    def configure_client(self, client: paramiko.SSHClient, username: str = "root", port: int = 22, timeout: int = 10) -> None:
        """Connect Paramiko SSHClient with RejectPolicy host key validation."""
        client.set_missing_host_key_policy(paramiko.RejectPolicy())
        auth_params = self.get_auth_params()
        client.connect(
            self.router_host,
            port=port,
            username=username,
            timeout=timeout,
            **auth_params
        )
