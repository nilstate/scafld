#!/usr/bin/env python3
"""Evaluate one JSON assertion expression for shell smoke tests."""

import json
import os
import sys


SAFE_BUILTINS = {
    "all": all,
    "any": any,
    "len": len,
    "max": max,
    "min": min,
    "set": set,
    "sorted": sorted,
    "sum": sum,
    "tuple": tuple,
}


def main():
    expression = sys.argv[1]
    message = sys.argv[2]
    data = json.loads(os.environ["JSON_PAYLOAD"])
    ok = eval(expression, {"__builtins__": SAFE_BUILTINS}, {"data": data})
    if not ok:
        raise SystemExit(message)


if __name__ == "__main__":
    main()
