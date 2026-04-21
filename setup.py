from pathlib import Path

from setuptools import setup


ROOT = Path(__file__).resolve().parent
VERSION = "1.4.2"


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
    url="https://github.com/nilstate/scafld",
    project_urls={
        "Source": "https://github.com/nilstate/scafld",
        "Issues": "https://github.com/nilstate/scafld/issues",
    },
    license="MIT",
    python_requires=">=3.9",
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
        "License :: OSI Approved :: MIT License",
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
