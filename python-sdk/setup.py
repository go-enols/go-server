"""Setup script for go-server Python SDK"""

from setuptools import setup, find_packages
import os

# Read the README file
here = os.path.abspath(os.path.dirname(__file__))
try:
    with open(os.path.join(here, 'README.md'), encoding='utf-8') as f:
        long_description = f.read()
except FileNotFoundError:
    long_description = "Python SDK for go-server distributed task scheduling system"

# Read requirements
with open(os.path.join(here, 'requirements.txt')) as f:
    requirements = [line.strip() for line in f if line.strip() and not line.startswith('#')]

setup(
    name="go-server-sdk",
    version="2.0.1",
    author="enols",
    author_email="2575169674@qq.com",
    description="Python SDK for go-server distributed task scheduling system",
    long_description=long_description,
    long_description_content_type="text/markdown",
    url="https://github.com/go-enols/go-server",
    packages=find_packages(),
    classifiers=[
        "Development Status :: 4 - Beta",
        "Intended Audience :: Developers",
        "License :: OSI Approved :: MIT License",
        "Operating System :: OS Independent",
        "Programming Language :: Python :: 3",
        "Programming Language :: Python :: 3.7",
        "Programming Language :: Python :: 3.8",
        "Programming Language :: Python :: 3.9",
        "Programming Language :: Python :: 3.10",
        "Programming Language :: Python :: 3.11",
        "Topic :: Software Development :: Libraries :: Python Modules",
        "Topic :: System :: Distributed Computing",
    ],
    python_requires=">=3.7",
    install_requires=requirements,
    extras_require={
        "dev": [
            "pytest>=6.0",
            "pytest-asyncio>=0.18.0",
            "black>=21.0.0",
            "flake8>=3.8.0",
            "mypy>=0.800",
        ],
    },
    keywords="distributed computing, task scheduling, websocket, rpc",
    project_urls={
        "Bug Reports": "https://github.com/go-enols/go-server/issues",
        "Source": "https://github.com/go-enols/go-server",
        "Documentation": "https://github.com/go-enols/go-server/blob/main/README.md",
    },
)