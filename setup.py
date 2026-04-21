import re
from pathlib import Path

from setuptools import setup


ROOT = Path(__file__).resolve().parent


def read_version():
    version_file = ROOT / "scafld" / "_version.py"
    text = version_file.read_text(encoding="utf-8")
    match = re.search(r'^__version__\s*=\s*"([^"]+)"\s*$', text, re.MULTILINE)
    if not match:
        raise RuntimeError(f"could not read __version__ from {version_file}")
    return match.group(1)


VERSION = read_version()


def runtime_data_files():
    files = []
    top_level = [
        "AGENTS.md",
        "CLAUDE.md",
        "CONVENTIONS.md",
        "README.md",
        "LICENSE",
        "install.sh",
        "cli/scafld",
    ]
    for rel in top_level:
        files.append(("share/scafld" if "/" not in rel else "share/scafld/cli", [str(ROOT / rel)]))

    ai_roots = [
        ROOT / ".ai" / "OPERATORS.md",
        ROOT / ".ai" / "README.md",
        ROOT / ".ai" / "config.yaml",
    ]
    for path in ai_roots:
        files.append(("share/scafld/.ai", [str(path)]))

    files.append(("share/scafld/scafld", [str(ROOT / "scafld" / "_version.py")]))

    for path in sorted((ROOT / ".ai" / "prompts").rglob("*")):
        if path.is_file():
            files.append(("share/scafld/.ai/prompts", [str(path)]))

    for path in sorted((ROOT / ".ai" / "schemas").rglob("*")):
        if path.is_file():
            files.append(("share/scafld/.ai/schemas", [str(path)]))

    specs_root = ROOT / ".ai" / "specs"
    include_specs = [
        specs_root / "README.md",
        specs_root / "examples" / "add-error-codes.yaml",
    ]
    for path in include_specs:
        dest = "share/scafld/.ai/specs" if path.parent == specs_root else "share/scafld/.ai/specs/examples"
        files.append((dest, [str(path)]))

    return files


setup(
    name="scafld",
    version=VERSION,
    description="Spec-driven development framework for AI coding agents",
    long_description=(ROOT / "README.md").read_text(encoding="utf-8"),
    long_description_content_type="text/markdown",
    author="0state",
    url="https://0state.com/scafld",
    project_urls={
        "Homepage": "https://0state.com/scafld",
        "Documentation": "https://0state.com/scafld/docs",
        "Source": "https://github.com/nilstate/scafld",
        "Issues": "https://github.com/nilstate/scafld/issues",
    },
    license="MIT",
    license_files=["LICENSE"],
    keywords=["ai", "cli", "developer-tools", "planning", "spec-driven"],
    python_requires=">=3.9",
    install_requires=["PyYAML>=6.0,<7"],
    packages=["scafld"],
    entry_points={
        "console_scripts": [
            "scafld=scafld.__main__:main",
        ]
    },
    data_files=runtime_data_files(),
    classifiers=[
        "Development Status :: 4 - Beta",
        "Environment :: Console",
        "Intended Audience :: Developers",
        "Programming Language :: Python :: 3",
        "Programming Language :: Python :: 3 :: Only",
        "Programming Language :: Python :: 3.9",
        "Programming Language :: Python :: 3.10",
        "Programming Language :: Python :: 3.11",
        "Programming Language :: Python :: 3.12",
        "Topic :: Software Development :: Build Tools",
        "Topic :: Software Development :: Quality Assurance",
    ],
)
