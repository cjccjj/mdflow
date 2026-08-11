#!/usr/bin/env python3
"""Run reproducible Markdown renderer benchmarks."""

import argparse
import hashlib
import json
import os
import signal
import shutil
import statistics
import subprocess
import sys
import time
from pathlib import Path


TARGETS = (
    ("1 MiB", 1024 * 1024),
    ("10 MiB", 10 * 1024 * 1024),
    ("100 MiB", 100 * 1024 * 1024),
)
TOOLS = ("mdflow", "glow", "mdcat", "streamdown")
REPOSITORY_ROOT = Path(__file__).resolve().parents[1]


class BenchmarkTimeout(Exception):
    def __init__(self, seconds):
        self.seconds = seconds


def resolve_tool(explicit, name, local_path=None):
    if explicit is not None:
        path = explicit
        if not path.is_file() or not os.access(path, os.X_OK):
            raise RuntimeError("benchmark executable is not runnable: %s" % path)
        return path.resolve()

    if local_path is not None:
        path = REPOSITORY_ROOT / local_path
        if path.is_file() and os.access(path, os.X_OK):
            return path.resolve()

    located = shutil.which(name)
    if located is not None:
        return Path(located).resolve()

    option = "--%s" % name
    raise RuntimeError(
        "could not find %s; pass %s or put it on PATH" % (name, option)
    )


def remove_generated_path(path):
    if path.is_symlink() or path.is_file():
        path.unlink()
    elif path.is_dir():
        shutil.rmtree(path)


def clean_output_dir(output_dir):
    output_dir.mkdir(parents=True, exist_ok=True)
    for name in ("benchmark.json", "benchmark.md", "inputs", "timings"):
        remove_generated_path(output_dir / name)


def report_progress(completed, total, workload, tool, phase, number, count):
    print(
        "benchmark: [%d/%d] %s %s %s %d/%d"
        % (completed, total, workload, tool, phase, number, count),
        flush=True,
    )


def report_timeout(workload, tool, phase, number, count, seconds):
    print(
        "benchmark: %s %s %s %d/%d didn't finish in %ss"
        % (workload, tool, phase, number, count, seconds),
        flush=True,
    )


def report_skipped(workload, tool, phase, number, count):
    print(
        "benchmark: %s %s %s %d/%d skipped (earlier timeout)"
        % (workload, tool, phase, number, count),
        flush=True,
    )


def sha256(path):
    digest = hashlib.sha256()
    with path.open("rb") as stream:
        while True:
            chunk = stream.read(1024 * 1024)
            if not chunk:
                break
            digest.update(chunk)
    return digest.hexdigest()


def make_workload(source, destination, target_size):
    data = source.read_bytes()
    if not data:
        raise RuntimeError("benchmark source is empty: %s" % source)

    destination.parent.mkdir(parents=True, exist_ok=True)
    full_copies, remainder = divmod(target_size, len(data))
    with destination.open("wb") as stream:
        for _ in range(full_copies):
            stream.write(data)

        if remainder:
            # End on a source newline instead of cutting a UTF-8 character or
            # Markdown construct in half. The exact byte count is recorded.
            partial_size = data.rfind(b"\n", 0, remainder) + 1
            if partial_size == 0:
                partial_size = remainder
            stream.write(data[:partial_size])

    return destination.stat().st_size


def run_command(argv, input_path, timing_path, timeout, environment):
    command = [
        "/usr/bin/time",
        "-f",
        "%e\t%M",
        "-o",
        str(timing_path),
    ] + argv

    with input_path.open("rb") as input_stream:
        process = subprocess.Popen(
            command,
            stdin=input_stream,
            stdout=subprocess.DEVNULL,
            stderr=subprocess.PIPE,
            env=environment,
            start_new_session=True,
        )

        deadline = time.monotonic() + timeout
        try:
            stderr = process.communicate(timeout=timeout)[1]
        except subprocess.TimeoutExpired:
            os.killpg(process.pid, signal.SIGTERM)
            remaining = max(0.0, deadline - time.monotonic())
            try:
                stderr = process.communicate(timeout=remaining)[1]
            except subprocess.TimeoutExpired:
                os.killpg(process.pid, signal.SIGKILL)
                stderr = process.communicate()[1]
            raise BenchmarkTimeout(timeout)

    if process.returncode != 0:
        message = stderr.decode("utf-8", "replace").strip()
        if not message:
            message = "no stderr output"
        raise RuntimeError(
            "command exited with status %s: %s\n%s"
            % (process.returncode, argv, message)
        )

    fields = timing_path.read_text(encoding="utf-8").strip().split()
    if len(fields) != 2:
        raise RuntimeError("unexpected /usr/bin/time output: %s" % fields)

    return {
        "wall_seconds": float(fields[0]),
        "max_rss_kib": int(fields[1]),
    }


def format_seconds(value):
    return "%.3f s" % value


def format_mib(kib):
    return "%.1f MiB" % (kib / 1024.0)


def format_bytes(size):
    if size >= 1024 * 1024:
        return "%.1f MiB" % (size / (1024.0 * 1024.0))
    return "%.1f KiB" % (size / 1024.0)


def bold_minimum(value, values, formatter):
    rendered = formatter(value)
    if value == min(values):
        return "**%s**" % rendered
    return rendered


def metric_cell(measurements, key, formatter, available, timeout):
    completed = [item[key] for item in measurements if item.get("status") == "ok"]
    if not completed:
        if any(item.get("status") == "timeout" for item in measurements):
            return "didn't finish in %ss" % timeout
        return "no result"

    value = statistics.median(completed)
    rendered = bold_minimum(value, available, formatter)
    timed_out = len(measurements) - len(completed)
    if timed_out:
        rendered += " (%d timeout)" % timed_out
    return rendered


def build_markdown(workloads, results, binaries, runs, warmups, timeout):
    lines = [
        "# Benchmark Results",
        "",
        "Workload: repeated copies of the checked-in benchmark source; output discarded; fixed 80-column settings.",
        "Measured runs per case: %d; warmup runs per case: %d." % (runs, warmups),
        "A timeout is reported as `didn't finish in %ss`; other execution errors fail the benchmark." % timeout,
        "",
        "| Markdown input | Metric | **mdflow** | **glow** | **mdcat** | **streamdown** |",
        "|:---|:---|---:|---:|---:|---:|",
    ]

    for workload in workloads:
        name = workload["name"]
        size = workload["actual_bytes"]
        values = {
            tool: results[name][tool] for tool in TOOLS
        }
        times = {
            tool: statistics.median(
                item["wall_seconds"]
                for item in values[tool]
                if item.get("status") == "ok"
            )
            for tool in values
            if any(item.get("status") == "ok" for item in values[tool])
        }
        rss = {
            tool: statistics.median(
                item["max_rss_kib"]
                for item in values[tool]
                if item.get("status") == "ok"
            )
            for tool in values
            if any(item.get("status") == "ok" for item in values[tool])
        }
        time_cells = [
            metric_cell(values[tool], "wall_seconds", format_seconds, list(times.values()), timeout)
            for tool in TOOLS
        ]
        rss_cells = [
            metric_cell(values[tool], "max_rss_kib", format_mib, list(rss.values()), timeout)
            for tool in TOOLS
        ]
        label = "**%s** (%d bytes)" % (name, size)
        lines.append("| %s | Median wall time | %s | %s | %s | %s |" % (label, *time_cells))
        lines.append("| | Median peak RSS | %s | %s | %s | %s |" % tuple(rss_cells))

    lines.extend(
        [
            "",
            "## Binary size",
            "",
            "| | **mdflow** | **glow** | **mdcat** | **streamdown** |",
            "|:---|---:|---:|---:|---:|",
            "| On-disk executable | %s | %s | %s | %s |"
            % tuple(format_bytes(binaries[tool]) for tool in TOOLS),
            "",
        ]
    )
    return "\n".join(lines)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--source",
        type=Path,
        default=REPOSITORY_ROOT / "assets/demo.md",
        help="Markdown source (default: assets/demo.md)",
    )
    parser.add_argument(
        "--mdflow",
        type=Path,
        help="mdflow executable (default: build/mdflow, then PATH)",
    )
    parser.add_argument(
        "--glow",
        type=Path,
        help="glow executable (default: PATH)",
    )
    parser.add_argument(
        "--mdcat",
        type=Path,
        help="mdcat executable (default: PATH)",
    )
    parser.add_argument(
        "--streamdown",
        type=Path,
        help="streamdown executable (default: PATH)",
    )
    parser.add_argument(
        "--output-dir",
        type=Path,
        default=Path("local/benchmark-results"),
        help="result directory (default: local/benchmark-results)",
    )
    parser.add_argument(
        "--runs", type=int, default=1,
        help="measured runs per renderer and input size (default: 1, max 3)",
    )
    parser.add_argument(
        "--warmups", type=int, default=1,
        help="warmup runs per renderer and input size (default: 1)",
    )
    parser.add_argument(
        "--timeout", type=int, default=100,
        help="per-command timeout in seconds (default: 100)",
    )
    args = parser.parse_args()

    if not 1 <= args.runs <= 3 or not 0 <= args.warmups <= 5 or args.timeout < 1:
        parser.error(
            "runs must be 1-3; warmups must be 0-5; timeout must be positive"
        )

    args.mdflow = resolve_tool(args.mdflow, "mdflow", Path("build/mdflow"))
    args.glow = resolve_tool(args.glow, "glow")
    args.mdcat = resolve_tool(args.mdcat, "mdcat")
    args.streamdown = resolve_tool(args.streamdown, "streamdown")
    clean_output_dir(args.output_dir)
    input_dir = args.output_dir / "inputs"
    timing_dir = args.output_dir / "timings"
    input_dir.mkdir(exist_ok=True)
    timing_dir.mkdir(exist_ok=True)

    specs = (
        # mdflow must run without pacing: typewriter delays would pollute
        # renderer wall-time measurements.
        ("mdflow", [str(args.mdflow), "--typewriter-off"]),
        ("glow", [str(args.glow), "--style", "dark", "--width", "80"]),
        ("mdcat", [str(args.mdcat), "--ansi", "--no-pager", "--columns", "80"]),
        ("streamdown", [str(args.streamdown), "-w", "80"]),
    )
    environment = os.environ.copy()
    environment.update(
        {
            "COLUMNS": "80",
            "LINES": "24",
            "TERM": "xterm-256color",
        }
    )
    environment.pop("NO_COLOR", None)

    total_commands = len(TARGETS) * len(specs) * (args.warmups + args.runs)
    print(
        "benchmark: %d workloads, %d renderers, %d warmups, %d runs, %ds timeout"
        % (len(TARGETS), len(specs), args.warmups, args.runs, args.timeout),
        flush=True,
    )

    workloads = []
    results = {}
    completed = 0
    gave_up = {tool: False for tool, _ in specs}
    for label, target_size in TARGETS:
        safe_label = label.replace(" ", "")
        input_path = input_dir / ("demo-%s.md" % safe_label)
        actual_size = make_workload(args.source, input_path, target_size)
        workload = {
            "name": label,
            "target_bytes": target_size,
            "actual_bytes": actual_size,
            "sha256": sha256(input_path),
            "path": str(input_path),
        }
        workloads.append(workload)
        results[label] = {tool: [] for tool, _ in specs}

        for tool, argv in specs:
            for warmup in range(args.warmups):
                completed += 1
                report_progress(
                    completed,
                    total_commands,
                    label,
                    tool,
                    "warmup",
                    warmup + 1,
                    args.warmups,
                )
                if gave_up[tool]:
                    report_skipped(label, tool, "warmup", warmup + 1, args.warmups)
                    continue
                timing_path = timing_dir / ("%s-%s-warmup-%d.time" % (safe_label, tool, warmup + 1))
                try:
                    run_command(argv, input_path, timing_path, args.timeout, environment)
                except BenchmarkTimeout as timeout:
                    gave_up[tool] = True
                    report_timeout(
                        label,
                        tool,
                        "warmup",
                        warmup + 1,
                        args.warmups,
                        timeout.seconds,
                    )

        for run in range(args.runs):
            for offset in range(len(specs)):
                tool, argv = specs[(run + offset) % len(specs)]
                completed += 1
                report_progress(
                    completed,
                    total_commands,
                    label,
                    tool,
                    "run",
                    run + 1,
                    args.runs,
                )
                if gave_up[tool]:
                    report_skipped(label, tool, "run", run + 1, args.runs)
                    results[label][tool].append({
                        "status": "timeout",
                        "timeout_seconds": args.timeout,
                        "run": run + 1,
                        "skipped": True,
                    })
                    continue
                timing_path = timing_dir / ("%s-%s-run-%d.time" % (safe_label, tool, run + 1))
                try:
                    measurement = run_command(argv, input_path, timing_path, args.timeout, environment)
                    measurement["status"] = "ok"
                except BenchmarkTimeout as timeout:
                    gave_up[tool] = True
                    report_timeout(
                        label,
                        tool,
                        "run",
                        run + 1,
                        args.runs,
                        timeout.seconds,
                    )
                    measurement = {
                        "status": "timeout",
                        "timeout_seconds": timeout.seconds,
                    }
                measurement["run"] = run + 1
                results[label][tool].append(measurement)

    binaries = {
        tool: path.stat().st_size
        for tool, path in (
            ("mdflow", args.mdflow),
            ("glow", args.glow),
            ("mdcat", args.mdcat),
            ("streamdown", args.streamdown),
        )
    }
    payload = {
        "source": str(args.source),
        "runs": args.runs,
        "warmups": args.warmups,
        "timeout_seconds": args.timeout,
        "workloads": workloads,
        "binaries": binaries,
        "results": results,
    }
    (args.output_dir / "benchmark.json").write_text(
        json.dumps(payload, indent=2) + "\n", encoding="utf-8"
    )
    (args.output_dir / "benchmark.md").write_text(
        build_markdown(
            workloads,
            results,
            binaries,
            args.runs,
            args.warmups,
            args.timeout,
        ),
        encoding="utf-8",
    )
    print("benchmark: wrote %s" % (args.output_dir / "benchmark.md"), flush=True)


if __name__ == "__main__":
    try:
        main()
    except (OSError, RuntimeError) as error:
        print("benchmark failed: %s" % error, file=sys.stderr)
        sys.exit(1)
