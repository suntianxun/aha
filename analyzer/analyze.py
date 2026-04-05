#!/usr/bin/env python3
"""Deep Python project analyzer using the ast module.

Usage: python3 analyze.py <project_dir>

Output: Newline-delimited JSON to stdout.
  - Progress lines: {"type": "progress", "phase": "...", "current": N, "total": N, "detail": "..."}
  - Result line:    {"type": "result", "files": [...], "patterns": [...]}
"""

import ast
import json
import os
import sys


def emit(obj):
    print(json.dumps(obj), flush=True)


def progress(phase, current, total, detail=""):
    emit({"type": "progress", "phase": phase, "current": current, "total": total, "detail": detail})


SKIP_DIRS = {".git", "__pycache__", ".tox", "node_modules", ".venv", "venv", ".eggs", "build", "dist", ".mypy_cache", ".pytest_cache"}


def find_py_files(project_dir):
    py_files = []
    for root, dirs, files in os.walk(project_dir):
        dirs[:] = [d for d in dirs if d not in SKIP_DIRS]
        for f in files:
            if f.endswith(".py"):
                full = os.path.join(root, f)
                rel = os.path.relpath(full, project_dir)
                py_files.append((full, rel))
    py_files.sort(key=lambda x: x[1])
    return py_files


def analyze_function(node):
    params = []
    for arg in node.args.args:
        param = arg.arg
        if arg.annotation:
            param += ": " + ast.unparse(arg.annotation)
        params.append(param)

    return_type = ""
    if node.returns:
        return_type = ast.unparse(node.returns)

    decorators = []
    for dec in node.decorator_list:
        decorators.append(ast.unparse(dec))

    loc = (node.end_lineno or node.lineno) - node.lineno + 1

    return {
        "name": node.name,
        "params": params,
        "return_type": return_type,
        "decorators": decorators,
        "loc": loc,
        "line_no": node.lineno,
    }


def analyze_class(node):
    bases = [ast.unparse(b) for b in node.bases]
    decorators = [ast.unparse(d) for d in node.decorator_list]

    methods = []
    # Get direct methods only (not nested)
    for item in node.body:
        if isinstance(item, (ast.FunctionDef, ast.AsyncFunctionDef)):
            methods.append(analyze_function(item))

    loc = (node.end_lineno or node.lineno) - node.lineno + 1

    return {
        "name": node.name,
        "bases": bases,
        "decorators": decorators,
        "methods": methods,
        "loc": loc,
        "line_no": node.lineno,
    }


def analyze_file(full_path, rel_path):
    try:
        with open(full_path, "r", encoding="utf-8", errors="replace") as f:
            source = f.read()
    except Exception:
        return None

    loc = source.count("\n") + (1 if source and not source.endswith("\n") else 0)

    try:
        tree = ast.parse(source, filename=rel_path)
    except SyntaxError:
        return {
            "rel_path": rel_path,
            "loc": loc,
            "classes": [],
            "functions": [],
            "imports": [],
            "all_exports": [],
            "constants": [],
        }

    classes = []
    functions = []
    imports = []
    all_exports = []
    constants = []

    for node in ast.iter_child_nodes(tree):
        if isinstance(node, ast.ClassDef):
            classes.append(analyze_class(node))

        elif isinstance(node, (ast.FunctionDef, ast.AsyncFunctionDef)):
            functions.append(analyze_function(node))

        elif isinstance(node, ast.Import):
            for alias in node.names:
                imports.append(alias.name)

        elif isinstance(node, ast.ImportFrom):
            module = node.module or ""
            level = node.level or 0
            prefix = "." * level
            if module:
                imports.append(prefix + module)
            elif prefix:
                for alias in node.names:
                    imports.append(prefix + alias.name)

        elif isinstance(node, ast.Assign):
            for target in node.targets:
                if isinstance(target, ast.Name):
                    if target.id == "__all__":
                        if isinstance(node.value, (ast.List, ast.Tuple)):
                            for elt in node.value.elts:
                                if isinstance(elt, ast.Constant) and isinstance(elt.value, str):
                                    all_exports.append(elt.value)
                    elif target.id.isupper() and not target.id.startswith("_"):
                        constants.append(target.id)

    return {
        "rel_path": rel_path,
        "loc": loc,
        "classes": classes,
        "functions": functions,
        "imports": imports,
        "all_exports": all_exports,
        "constants": constants,
    }


def detect_patterns(files):
    patterns = []

    for f in files:
        rel = f["rel_path"]
        for cls in f.get("classes", []):
            for base in cls.get("bases", []):
                if base in ("ABC", "abc.ABC"):
                    patterns.append({
                        "pattern": "Abstract Base Class",
                        "location": f"{rel}:{cls['name']}",
                        "detail": f"Inherits from {base}",
                    })
                if base == "Protocol" or base == "typing.Protocol":
                    patterns.append({
                        "pattern": "Protocol",
                        "location": f"{rel}:{cls['name']}",
                        "detail": "Implements typing.Protocol",
                    })

            for dec in cls.get("decorators", []):
                if "dataclass" in dec:
                    patterns.append({
                        "pattern": "Dataclass",
                        "location": f"{rel}:{cls['name']}",
                        "detail": f"@{dec}",
                    })
                if "pydantic" in dec.lower() or dec in ("BaseModel",):
                    patterns.append({
                        "pattern": "Pydantic Model",
                        "location": f"{rel}:{cls['name']}",
                        "detail": f"@{dec}",
                    })

            for base in cls.get("bases", []):
                if "BaseModel" in base:
                    patterns.append({
                        "pattern": "Pydantic Model",
                        "location": f"{rel}:{cls['name']}",
                        "detail": f"Inherits from {base}",
                    })
                if "Enum" in base or "enum." in base:
                    patterns.append({
                        "pattern": "Enum",
                        "location": f"{rel}:{cls['name']}",
                        "detail": f"Inherits from {base}",
                    })
                if "Exception" in base or "Error" in base:
                    patterns.append({
                        "pattern": "Custom Exception",
                        "location": f"{rel}:{cls['name']}",
                        "detail": f"Inherits from {base}",
                    })
                if "type" in base.lower() and "meta" in base.lower():
                    patterns.append({
                        "pattern": "Metaclass",
                        "location": f"{rel}:{cls['name']}",
                        "detail": f"Uses metaclass: {base}",
                    })

            method_names = [m["name"] for m in cls.get("methods", [])]
            if "__new__" in method_names and "__init__" in method_names:
                patterns.append({
                    "pattern": "Possible Singleton",
                    "location": f"{rel}:{cls['name']}",
                    "detail": "Defines both __new__ and __init__",
                })

        for func in f.get("functions", []):
            for dec in func.get("decorators", []):
                if dec in ("property", "staticmethod", "classmethod", "abstractmethod"):
                    continue
                if dec.startswith("app.") or dec.startswith("router."):
                    patterns.append({
                        "pattern": "Route Handler",
                        "location": f"{rel}:{func['name']}",
                        "detail": f"@{dec}",
                    })
                elif dec.startswith("click.") or dec.startswith("typer."):
                    patterns.append({
                        "pattern": "CLI Command",
                        "location": f"{rel}:{func['name']}",
                        "detail": f"@{dec}",
                    })
                elif dec.startswith("pytest."):
                    patterns.append({
                        "pattern": "Pytest Fixture/Mark",
                        "location": f"{rel}:{func['name']}",
                        "detail": f"@{dec}",
                    })

    return patterns


def main():
    if len(sys.argv) != 2:
        print("Usage: analyze.py <project_dir>", file=sys.stderr)
        sys.exit(1)

    project_dir = os.path.abspath(sys.argv[1])

    progress("scan", 0, 0, "Scanning for Python files...")
    py_files = find_py_files(project_dir)
    total = len(py_files)
    progress("scan", total, total, f"Found {total} Python files")

    files = []
    for i, (full_path, rel_path) in enumerate(py_files):
        progress("parse", i + 1, total, rel_path)
        result = analyze_file(full_path, rel_path)
        if result:
            files.append(result)

    progress("patterns", 0, 1, "Detecting patterns...")
    patterns = detect_patterns(files)
    progress("patterns", 1, 1, f"Found {len(patterns)} patterns")

    emit({"type": "result", "files": files, "patterns": patterns})


if __name__ == "__main__":
    main()
