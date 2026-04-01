"""Cryptographic utilities for decrypting BYOK API keys.

API keys stored in the database are encrypted with AES-256-GCM.
The ciphertext and 16-byte authentication tag are stored concatenated
in the ``api_key_encrypted`` column, and the nonce/IV is stored
separately in ``api_key_iv``.
"""

from __future__ import annotations

import base64

import structlog
from cryptography.hazmat.primitives.ciphers.aead import AESGCM

logger = structlog.get_logger(__name__)


def decrypt_api_key(encrypted: bytes, iv: bytes, key_b64: str) -> str:
    """Decrypt an AES-256-GCM encrypted API key.

    Args:
        encrypted: Concatenated ciphertext + 16-byte authentication tag
            as stored in the ``api_key_encrypted`` DB column.
        iv: The 12-byte nonce stored in the ``api_key_iv`` DB column.
        key_b64: Base64-encoded 32-byte AES key from the
            ``RAVEN_ENCRYPTION_KEY`` environment variable.

    Returns:
        The decrypted API key as a UTF-8 string.

    Raises:
        ValueError: If the key is not 32 bytes after base64-decoding.
        cryptography.exceptions.InvalidTag: If the ciphertext has been
            tampered with or the wrong key/IV was used.
    """
    key = base64.b64decode(key_b64)
    if len(key) != 32:
        raise ValueError(
            f"Encryption key must be 32 bytes after base64 decoding, got {len(key)} bytes"
        )

    aesgcm = AESGCM(key)
    plaintext = aesgcm.decrypt(iv, encrypted, None)

    logger.debug("decrypt_api_key_success", iv_len=len(iv), ciphertext_len=len(encrypted))
    return plaintext.decode("utf-8")
