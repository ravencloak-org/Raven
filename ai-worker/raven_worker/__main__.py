"""Entry point for running the Raven AI Worker: python -m raven_worker."""

import asyncio

from raven_worker.server import serve


def main() -> None:
    asyncio.run(serve())


if __name__ == "__main__":
    main()
