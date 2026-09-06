#!/usr/bin/env python3
"""Compare value-probe acceptance and identity semantics across revisions."""

from __future__ import annotations

import argparse
import json
from collections import defaultdict
from pathlib import Path
import sys


Key = tuple[str, str]

EXPECTED_ACCEPTANCE_DELTA: frozenset[Key] = frozenset(
    {
        ("duration", "P999999999999999999999999999Y"),
    }
)


def load_results(path: Path) -> dict[Key, dict[str, object]]:
    results: dict[Key, dict[str, object]] = {}
    with path.open(encoding="utf-8") as stream:
        for line_number, line in enumerate(stream, 1):
            if not line.strip():
                continue
            result = json.loads(line)
            key = (result["Type"], result["Text"])
            if key in results:
                raise ValueError(f"duplicate probe key at {path}:{line_number}: {key!r}")
            results[key] = result
    return results


def equivalence_partition(
    results: dict[Key, dict[str, object]], keys: set[Key]
) -> frozenset[frozenset[Key]]:
    groups: dict[str, set[Key]] = defaultdict(set)
    for key in keys:
        identity = results[key].get("Identity")
        if not isinstance(identity, str):
            raise ValueError(f"accepted probe has no identity: {key!r}")
        groups[identity].add(key)
    return frozenset(frozenset(group) for group in groups.values())


def format_keys(keys: set[Key] | frozenset[Key]) -> str:
    return ", ".join(f"{kind}:{text!r}" for kind, text in sorted(keys))


def compare(baseline_path: Path, rewrite_path: Path) -> int:
    baseline = load_results(baseline_path)
    rewrite = load_results(rewrite_path)
    missing = set(baseline) - set(rewrite)
    added = set(rewrite) - set(baseline)
    if missing or added:
        if missing:
            print(f"missing rewrite keys ({len(missing)}): {format_keys(missing)}")
        if added:
            print(f"added rewrite keys ({len(added)}): {format_keys(added)}")
        return 1

    acceptance_delta = {
        key for key in baseline if baseline[key]["Valid"] != rewrite[key]["Valid"]
    }
    unsupported_delta = {
        key
        for key in baseline
        if baseline[key]["Unsupported"] != rewrite[key]["Unsupported"]
    }
    unexpected_acceptance = acceptance_delta - EXPECTED_ACCEPTANCE_DELTA
    missing_expected = EXPECTED_ACCEPTANCE_DELTA - acceptance_delta

    jointly_accepted = {
        key
        for key in baseline
        if baseline[key]["Valid"] and rewrite[key]["Valid"]
    }
    baseline_partition = equivalence_partition(baseline, jointly_accepted)
    rewrite_partition = equivalence_partition(rewrite, jointly_accepted)
    identity_text_delta = {
        key
        for key in jointly_accepted
        if baseline[key]["Identity"] != rewrite[key]["Identity"]
    }

    print(f"baseline keys: {len(baseline)}")
    print(f"rewrite keys: {len(rewrite)}")
    print(f"jointly accepted: {len(jointly_accepted)}")
    print(f"acceptance deltas: {len(acceptance_delta)}")
    if acceptance_delta:
        print(f"  {format_keys(acceptance_delta)}")
    print(f"unsupported deltas: {len(unsupported_delta)}")
    print(f"identity text changes among jointly accepted: {len(identity_text_delta)}")
    print(f"identity partitions: {'equal' if baseline_partition == rewrite_partition else 'DIFFER'}")

    failures: list[str] = []
    if unexpected_acceptance:
        failures.append(
            f"unexpected acceptance deltas ({len(unexpected_acceptance)}): "
            + format_keys(unexpected_acceptance)
        )
    if missing_expected:
        failures.append(
            f"expected acceptance deltas missing ({len(missing_expected)}): "
            + format_keys(missing_expected)
        )
    if unsupported_delta:
        failures.append(
            f"unsupported deltas ({len(unsupported_delta)}): "
            + format_keys(unsupported_delta)
        )
    if baseline_partition != rewrite_partition:
        failures.append("identity equivalence partition changed")
    if failures:
        for failure in failures:
            print(f"ERROR: {failure}")
        return 1
    return 0


def main() -> int:
    probe_dir = Path(__file__).resolve().parent
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--baseline",
        type=Path,
        default=probe_dir / "baseline-identity.jsonl",
    )
    parser.add_argument(
        "--rewrite",
        type=Path,
        default=probe_dir / "rewrite-identity.jsonl",
    )
    args = parser.parse_args()
    try:
        return compare(args.baseline, args.rewrite)
    except (OSError, KeyError, TypeError, ValueError, json.JSONDecodeError) as error:
        print(f"ERROR: {error}", file=sys.stderr)
        return 2


if __name__ == "__main__":
    raise SystemExit(main())
