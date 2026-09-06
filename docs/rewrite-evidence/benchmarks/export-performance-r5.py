#!/usr/bin/env python3
"""Export paired benchmark samples for review.

This deliberately does not run benchmarks or calculate an acceptance result.  It
only reads complete samples, checks that their row sets are pairable, and writes
a reviewable CSV/JSON/Markdown packet. The default status is historical; an
export never declares performance parity.
"""

from __future__ import annotations

import argparse
import csv
from dataclasses import dataclass
from datetime import datetime, timezone
from decimal import Decimal, InvalidOperation
import hashlib
import json
from pathlib import Path
import re
import shutil
import sys
from typing import Iterable


ROUNDS = 6
EXPECTED_BENCHTIME = "200ms"
R5_GROUPS = (
    "public",
    "format",
    "regex",
    "value",
    "stream",
    "duration",
    "validate",
    "concurrent",
    "namespace",
    "lex",
    "uri",
)
R3_GROUPS = ()

# Benchmark names are whitespace-free.  Go may append a CPU label to the name
# (for example BenchmarkValidateConcurrent-8); the label is retained as a
# separate CSV field so the two concurrent rows do not collapse.
BENCHMARK_LINE = re.compile(
    r"^(?P<name>\S+)\s+"
    r"(?P<iterations>\d+)\s+"
    r"(?P<time>[0-9]+(?:\.[0-9]+)?(?:[eE][+-]?[0-9]+)?)\s+"
    r"(?P<unit>ns|us|µs|μs|ms|s)/op"
    r"(?:\s+(?P<throughput>[0-9]+(?:\.[0-9]+)?(?:[eE][+-]?[0-9]+)?)\s+MB/s)?\s+"
    r"(?P<bytes>[0-9]+(?:\.[0-9]+)?(?:[eE][+-]?[0-9]+)?)\s+B/op\s+"
    r"(?P<allocs>[0-9]+(?:\.[0-9]+)?(?:[eE][+-]?[0-9]+)?)\s+allocs/op\s*$"
)
MATRIX_LINE = re.compile(
    r"^(?P<prefix>r[0-9]+)-(?P<group>[a-z-]+) round (?P<round>[1-6])/6 "
    r"(?P<source>baseline|current): exit (?P<exit>[0-9]+)\s*$"
)
CPU_SUFFIX = re.compile(r"^(?P<name>.*)-(?P<cpu>[0-9]+)$")


@dataclass(frozen=True)
class Sample:
    prefix: str
    group: str
    source: str
    round: int
    source_cycle: int
    execution_index: int
    benchmark: str
    cpu: str
    raw_benchmark: str
    iterations: int
    ns_op: str
    bytes_op: str
    allocs_op: str
    comparison: str
    note: str
    raw_file: str


class ExportError(RuntimeError):
    pass


def decimal(value: str, *, field: str, path: Path) -> Decimal:
    try:
        parsed = Decimal(value)
    except InvalidOperation as exc:
        raise ExportError(f"{path}: invalid {field} value {value!r}") from exc
    if not parsed.is_finite() or parsed < 0:
        raise ExportError(f"{path}: invalid {field} value {value!r}")
    return parsed


def decimal_text(value: Decimal) -> str:
    text = format(value, "f")
    if "." in text:
        text = text.rstrip("0").rstrip(".")
    return text or "0"


def sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as stream:
        for block in iter(lambda: stream.read(1024 * 1024), b""):
            digest.update(block)
    return digest.hexdigest()


def parse_headers(lines: Iterable[str], path: Path) -> dict[str, str]:
    headers: dict[str, str] = {}
    for line in lines:
        for key in ("goos", "goarch", "cpu"):
            prefix = f"{key}:"
            if line.startswith(prefix):
                value = line[len(prefix) :].strip()
                if value:
                    headers[key] = value
    missing = [key for key in ("goos", "goarch", "cpu") if key not in headers]
    if missing:
        raise ExportError(f"{path}: incomplete benchmark header; missing {', '.join(missing)}")
    return headers


def normalize_benchmark(
    group: str,
    source: str,
    raw_name: str,
) -> tuple[str, str, str | None]:
    """Return canonical name, CPU label, and an optional drop reason."""

    match = CPU_SUFFIX.match(raw_name)
    if match:
        benchmark = match.group("name")
        cpu = match.group("cpu")
    else:
        benchmark = raw_name
        cpu = ""

    if group == "validate" and source == "baseline":
        if benchmark.endswith("/generic_match") or benchmark.endswith("/generic_miss"):
            return benchmark, cpu, "retired baseline generic identity adapter"
        if benchmark.endswith("/direct_match"):
            benchmark = benchmark[: -len("direct_match")] + "compiled_match"
        elif benchmark.endswith("/direct_miss"):
            benchmark = benchmark[: -len("direct_miss")] + "compiled_miss"

    return benchmark, cpu, None


def parse_samples(
    path: Path,
    *,
    prefix: str,
    group: str,
    source: str,
    round_number: int,
    source_cycle: int,
    execution_index: int,
) -> tuple[list[Sample], dict[str, str], list[dict[str, str]]]:
    if not path.is_file():
        raise ExportError(f"missing benchmark sample: {path}")
    text = path.read_text()
    if "\x00" in text:
        raise ExportError(f"{path}: sample is not text")
    lines = text.splitlines()
    if "FAIL" in lines or not any(line == "PASS" for line in lines):
        raise ExportError(f"{path}: sample is incomplete or failed (missing PASS)")

    headers = parse_headers(lines, path)
    samples: list[Sample] = []
    dropped: list[dict[str, str]] = []
    seen: set[tuple[str, str]] = set()
    for line in lines:
        match = BENCHMARK_LINE.match(line)
        if match is None:
            continue
        raw_name = match.group("name")
        benchmark, cpu, drop_reason = normalize_benchmark(group, source, raw_name)
        key = (benchmark, cpu)
        if drop_reason is not None:
            dropped.append(
                {
                    "group": group,
                    "source": source,
                    "round": str(round_number),
                    "benchmark": raw_name,
                    "reason": drop_reason,
                    "file": path.name,
                }
            )
            continue
        if key in seen:
            raise ExportError(f"{path}: duplicate benchmark row after normalization: {raw_name}")
        seen.add(key)

        iterations = int(match.group("iterations"))
        if iterations <= 0:
            raise ExportError(f"{path}: non-positive benchmark iteration count for {raw_name}")
        time_value = decimal(match.group("time"), field="time", path=path)
        time_scale = {"ns": 1, "us": 1_000, "µs": 1_000, "μs": 1_000, "ms": 1_000_000, "s": 1_000_000_000}[match.group("unit")]
        ns_op = decimal_text(time_value * time_scale)
        bytes_op = decimal_text(decimal(match.group("bytes"), field="B/op", path=path))
        allocs_op = decimal_text(decimal(match.group("allocs"), field="allocs/op", path=path))

        comparison = "paired"
        note = ""
        if group == "value" and benchmark == "BenchmarkPublishedRawUnionLateMember":
            comparison = "diagnostic"
            note = "old bool+error union API versus current typed Value+error API; exclude from acceptance"
        elif group == "public" and benchmark == "BenchmarkSessionValidateUnionLateMember":
            note = "matching public API workload added in both snapshots"

        samples.append(
            Sample(
                prefix=prefix,
                group=group,
                source=source,
                round=round_number,
                source_cycle=source_cycle,
                execution_index=execution_index,
                benchmark=benchmark,
                cpu=cpu,
                raw_benchmark=raw_name,
                iterations=iterations,
                ns_op=ns_op,
                bytes_op=bytes_op,
                allocs_op=allocs_op,
                comparison=comparison,
                note=note,
                raw_file=path.name,
            )
        )

    if not samples:
        raise ExportError(f"{path}: no benchmark rows")
    return samples, headers, dropped


def read_matrix(path: Path, prefix: str, groups: tuple[str, ...]) -> list[tuple[str, int, str, int]]:
    if not path.is_file():
        raise ExportError(f"missing matrix log: {path}")
    events: list[tuple[str, int, str, int]] = []
    expected = {(group, round_number, source) for group in groups for round_number in range(1, ROUNDS + 1) for source in ("baseline", "current")}
    seen: set[tuple[str, int, str]] = set()
    for execution_index, line in enumerate(path.read_text().splitlines(), start=1):
        match = MATRIX_LINE.match(line)
        if match is None:
            continue
        if match.group("prefix") != prefix:
            continue
        group = match.group("group")
        round_number = int(match.group("round"))
        source = match.group("source")
        key = (group, round_number, source)
        if group not in groups:
            if prefix == "r3" and group in R5_GROUPS:
                continue
            raise ExportError(f"{path}: unexpected {prefix} matrix group {group!r}")
        if int(match.group("exit")) != 0:
            raise ExportError(f"{path}: failed sample: {line}")
        if key in seen:
            raise ExportError(f"{path}: duplicate matrix event {key}")
        seen.add(key)
        events.append((group, round_number, source, execution_index))
    if seen != expected:
        missing = sorted(expected - seen)
        extra = sorted(seen - expected)
        raise ExportError(f"{path}: incomplete matrix log; missing={missing!r} extra={extra!r}")
    return events


def read_metadata(path: Path, expected_group: str) -> dict:
    if not path.is_file():
        raise ExportError(f"missing benchmark metadata: {path}")
    try:
        metadata = json.loads(path.read_text())
    except json.JSONDecodeError as exc:
        raise ExportError(f"{path}: invalid JSON") from exc
    if metadata.get("group") != expected_group:
        raise ExportError(f"{path}: metadata group is {metadata.get('group')!r}, expected {expected_group!r}")
    if metadata.get("rounds") != ROUNDS or metadata.get("benchtime") != EXPECTED_BENCHTIME:
        raise ExportError(f"{path}: expected six 200ms rounds")
    if "separate process" not in str(metadata.get("cpu_execution", "")):
        raise ExportError(f"{path}: CPU execution is not recorded as separate-process sampling")
    if not isinstance(metadata.get("binary_sha256"), dict):
        raise ExportError(f"{path}: missing binary SHA-256 metadata")
    return metadata


def validate_pair_rows(samples: list[Sample], groups: tuple[str, ...]) -> None:
    by_round: dict[tuple[str, int, str], set[tuple[str, str]]] = {}
    for sample in samples:
        by_round.setdefault((sample.group, sample.round, sample.source), set()).add((sample.benchmark, sample.cpu))
    for group in groups:
        for round_number in range(1, ROUNDS + 1):
            baseline = by_round[(group, round_number, "baseline")]
            current = by_round[(group, round_number, "current")]
            if baseline != current:
                raise ExportError(
                    f"{group} round {round_number}: benchmark rows differ after normalization; "
                    f"baseline-only={sorted(baseline - current)!r} current-only={sorted(current - baseline)!r}"
                )


def collect(
    input_dir: Path,
    *,
    prefix: str,
    baseline_revision: str | None,
    current_revision: str | None,
    status: str,
) -> tuple[list[Sample], dict, list[dict[str, str]]]:
    final_perf = input_dir.resolve()
    group_names = R5_GROUPS
    groups = [(prefix, group) for group in group_names]
    matrix_events: dict[tuple[str, int, str], tuple[int, int]] = {}
    matrix_path = final_perf / f"{prefix}-matrix.log"
    for source_group, round_number, source, execution_index in read_matrix(matrix_path, prefix, group_names):
        prior = matrix_events.get((source_group, round_number, source))
        if prior is not None:
            raise ExportError(f"duplicate matrix event for {source_group} round {round_number} {source}")
        matrix_events[(source_group, round_number, source)] = (execution_index, 0)

    all_samples: list[Sample] = []
    all_dropped: list[dict[str, str]] = []
    group_metadata: dict[str, dict] = {}
    headers_seen: set[tuple[str, str, str]] = set()
    input_hashes: dict[str, str] = {}
    for prefix, group in groups:
        group_key = f"{prefix}-{group}"
        metadata_path = final_perf / f"{group_key}-metadata.json"
        metadata = read_metadata(metadata_path, group_key)
        group_metadata[group_key] = {
            "binary_sha256": metadata["binary_sha256"],
            "go_version": metadata.get("go_version"),
            "source_commits": metadata.get("source_commits"),
            "source_status": metadata.get("source_status"),
            "observed_test_processes": metadata.get("observed_test_processes"),
            "cpu": metadata.get("cpu"),
            "cpu_execution": metadata.get("cpu_execution"),
            "metadata_sha256": sha256(metadata_path),
        }
        input_hashes[str(metadata_path.relative_to(final_perf))] = sha256(metadata_path)
        for round_number in range(1, ROUNDS + 1):
            for source in ("baseline", "current"):
                path = final_perf / f"{group_key}-{round_number}-{source}.txt"
                execution_index = matrix_events[(group, round_number, source)][0]
                source_cycle = (round_number - 1) * 2 + (1 if source == "baseline" else 2)
                parsed, headers, dropped = parse_samples(
                    path,
                    prefix=prefix,
                    group=group,
                    source=source,
                    round_number=round_number,
                    source_cycle=source_cycle,
                    execution_index=execution_index,
                )
                headers_seen.add((headers["goos"], headers["goarch"], headers["cpu"]))
                all_samples.extend(parsed)
                all_dropped.extend(dropped)
                input_hashes[str(path.relative_to(final_perf))] = sha256(path)

    if len(headers_seen) != 1:
        raise ExportError(f"benchmark platform headers differ: {sorted(headers_seen)!r}")
    validate_pair_rows(all_samples, group_names)

    first_go_version = next(iter(group_metadata.values()))["go_version"]
    if any(info["go_version"] != first_go_version for info in group_metadata.values()):
        raise ExportError("benchmark toolchain versions differ across groups")

    headers = next(iter(headers_seen))
    source_commits = next(iter(group_metadata.values())).get("source_commits") or {}
    baseline_revision = baseline_revision or source_commits.get("baseline")
    current_revision = current_revision or source_commits.get("current")
    if not baseline_revision or not current_revision:
        raise ExportError("source revisions are absent; pass --baseline-revision and --current-revision")
    export_metadata = {
        "status": status,
        "assessment": f"{status}; this export does not declare performance parity",
        "baseline_revision": baseline_revision,
        "current_revision": current_revision,
        "toolchain": first_go_version,
        "platform": {"goos": headers[0], "goarch": headers[1], "cpu": headers[2]},
        "sampling": {
            "rounds": ROUNDS,
            "benchtime": EXPECTED_BENCHTIME,
            "order": "alternating baseline/current per group; source cycle is preserved",
            "cpu_execution": "separate process per CPU setting",
            "groups": [f"{prefix}-{group}" for group in group_names],
        },
        "groups": group_metadata,
        "row_count": len(all_samples),
        "dropped_rows": all_dropped,
        "input_sha256": input_hashes,
        "normalizations": {
            "validate": {
                "baseline": {"/direct_match": "/compiled_match", "/direct_miss": "/compiled_miss"},
                "drop": ["/generic_match", "/generic_miss"],
            },
            "diagnostic": {
                f"{prefix}-value/BenchmarkPublishedRawUnionLateMember": "old bool+error API versus current typed Value+error API",
                f"{prefix}-public/BenchmarkSessionValidateUnionLateMember": "matching public API workload",
            },
        },
        "generated_at": datetime.now(timezone.utc).replace(microsecond=0).isoformat().replace("+00:00", "Z"),
    }
    return all_samples, export_metadata, all_dropped


def write_outputs(
    export_dir: Path,
    samples: list[Sample],
    metadata: dict,
    dropped: list[dict[str, str]],
    *,
    prefix: str,
) -> None:
    temporary = export_dir.with_name(export_dir.name + ".tmp")
    if temporary.exists():
        shutil.rmtree(temporary)
    temporary.mkdir(parents=True)
    try:
        with (temporary / "samples.csv").open("w", newline="") as stream:
            fields = [
                "prefix",
                "group",
                "source",
                "round",
                "source_cycle",
                "execution_index",
                "benchmark",
                "cpu",
                "raw_benchmark",
                "iterations",
                "ns_op",
                "bytes_op",
                "allocs_op",
                "comparison",
                "note",
                "raw_file",
            ]
            writer = csv.DictWriter(stream, fieldnames=fields)
            writer.writeheader()
            for sample in samples:
                writer.writerow({field: getattr(sample, field) for field in fields})

        (temporary / "metadata.json").write_text(json.dumps(metadata, indent=2, sort_keys=True) + "\n")

        group_counts: dict[str, int] = {}
        paired_rows: set[tuple[str, str, str]] = set()
        diagnostics: list[Sample] = []
        for sample in samples:
            group_counts[sample.group] = group_counts.get(sample.group, 0) + 1
            paired_rows.add((sample.group, sample.benchmark, sample.cpu))
            if sample.comparison == "diagnostic":
                diagnostics.append(sample)
        status = metadata["status"]
        lines = [
            f"# Performance export ({status})",
            "",
            f"Status: {status}. The rewrite performance goal remains open; this packet",
            "does not declare parity, regression, or acceptance.",
            "",
            "The export contains six complete alternating 200 ms samples for each",
            f"{prefix.upper()} group. `samples.csv` preserves group, baseline/current source, round,",
            "source-cycle, matrix execution order, benchmark, CPU label, and raw",
            "sample filename. `metadata.json` records revisions, hashes, toolchain,",
            "platform, CPU, sampling settings, normalizations, and dropped rows.",
            "",
            "## Coverage",
            "",
            "| group | samples | distinct paired rows |",
            "| --- | ---: | ---: |",
        ]
        for group in [f"{prefix}-{name}" for name in R5_GROUPS]:
            lines.append(
                f"| `{group}` | {sum(1 for sample in samples if f'{sample.prefix}-{sample.group}' == group)} | "
                f"{sum(1 for key in paired_rows if key[0] == group.removeprefix(prefix + '-'))} |"
            )
        lines.extend(
            [
                "",
                "The validate group maps baseline `/direct_match` and `/direct_miss`",
                "to current `/compiled_match` and `/compiled_miss`; baseline generic",
                "adapter rows are dropped. No other row mismatch is accepted.",
                "",
                "The raw `BenchmarkPublishedRawUnionLateMember` value row is retained",
                "as a diagnostic because the baseline returns bool+error while the",
                "current API returns typed `Value`+error. The public",
                "`BenchmarkSessionValidateUnionLateMember` row is a matching API",
                "workload and is included as a normal paired row.",
                "",
                f"Dropped baseline rows: {len(dropped)}. Distinct row identities: {len(paired_rows)}.",
                "",
                "## Review boundary",
                "",
                "Use the raw samples and this normalized CSV for review. Do not use the",
                "diagnostic raw-union row as an acceptance ratio. A final conclusion",
                "still requires the complete performance assessment, including the",
                "bounded-retention evidence required by `rewrite-plan.md`.",
            ]
        )
        (temporary / "summary.md").write_text("\n".join(lines) + "\n")

        if export_dir.exists():
            shutil.rmtree(export_dir)
        temporary.rename(export_dir)
    except Exception:
        shutil.rmtree(temporary, ignore_errors=True)
        raise


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--repo",
        type=Path,
        default=Path(__file__).resolve().parents[2],
        help="repository root (default: inferred from this script)",
    )
    parser.add_argument(
        "--input-dir",
        type=Path,
        help="directory containing <prefix>-matrix.log and sample files",
    )
    parser.add_argument("--output-dir", type=Path, help="directory for the exported packet")
    parser.add_argument("--prefix", default="r5", help="sample prefix, such as r5 or r6")
    parser.add_argument("--baseline-revision", help="override the baseline revision recorded in metadata")
    parser.add_argument("--current-revision", help="override the current revision recorded in metadata")
    parser.add_argument(
        "--status",
        choices=("historical", "draft"),
        default="historical",
        help="provenance label; neither status declares performance parity",
    )
    args = parser.parse_args()
    repo = args.repo.resolve()
    input_dir = (args.input_dir or repo / ".lab" / "rewrite" / "final-perf").resolve()
    output_dir = (args.output_dir or repo / ".lab" / "rewrite" / f"performance-export-{args.prefix}").resolve()
    try:
        samples, metadata, dropped = collect(
            input_dir,
            prefix=args.prefix,
            baseline_revision=args.baseline_revision,
            current_revision=args.current_revision,
            status=args.status,
        )
        write_outputs(output_dir, samples, metadata, dropped, prefix=args.prefix)
    except (OSError, ExportError, KeyError) as exc:
        print(f"export-performance: {exc}", file=sys.stderr)
        return 1
    print(f"export-performance: wrote {len(samples)} rows to {output_dir}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
