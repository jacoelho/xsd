#!/usr/bin/env python3
"""Reproduce the paired matrix with prepared baseline and rewrite worktrees."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import subprocess
from datetime import datetime, timezone


def run(command, **kwargs):
    return subprocess.run(command, check=True, text=True, stdout=subprocess.PIPE,
                          stderr=subprocess.STDOUT, **kwargs).stdout


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--baseline', required=True, type=Path)
    parser.add_argument('--current', required=True, type=Path)
    parser.add_argument('--output', required=True, type=Path)
    parser.add_argument('--rounds', type=int, default=6)
    parser.add_argument('--benchtime', default='200ms')
    parser.add_argument('--group', action='append', help='only selected manifest groups')
    args = parser.parse_args()
    if args.rounds < 1:
        parser.error('--rounds must be positive')
    output = args.output.resolve()
    output.mkdir(parents=True, exist_ok=False)
    roots = {'baseline': args.baseline.resolve(), 'current': args.current.resolve()}
    manifest = json.loads(Path(__file__).with_name('manifest.json').read_text())
    if args.group:
        unknown = set(args.group) - {item['group'] for item in manifest}
        if unknown:
            parser.error(f'unknown groups: {sorted(unknown)}')
        manifest = [item for item in manifest if item['group'] in args.group]
    metadata = dict(started=datetime.now(timezone.utc).isoformat(),
                    go_version=run(['go', 'version']).strip(), rounds=args.rounds,
                    benchtime=args.benchtime, manifest=manifest,
                    cpu_execution='one process per CPU setting; GOMAXPROCS equals -test.cpu',
                    revisions={}, source_status={}, binary_sha256={})
    for role, root in roots.items():
        metadata['revisions'][role] = run(['git', 'rev-parse', 'HEAD'], cwd=root).strip()
        metadata['source_status'][role] = run(['git', 'status', '--porcelain'], cwd=root).splitlines()
    binaries = {}
    for item in manifest:
        for role, root in roots.items():
            package = item[role + '_package']
            key = (role, package)
            if key in binaries:
                continue
            name = role + '-' + ('public' if package == '.' else package.rsplit('/', 1)[-1])
            binary = output / (name + '.test')
            run(['go', 'test', '-c', package, '-o', str(binary)], cwd=root)
            binaries[key] = binary
            metadata['binary_sha256'][name] = hashlib.sha256(binary.read_bytes()).hexdigest()
    (output / 'metadata.json').write_text(json.dumps(metadata, indent=2) + '\n')
    for item in manifest:
        group = item['group']
        for index in range(args.rounds):
            roles = ('baseline', 'current') if index % 2 == 0 else ('current', 'baseline')
            for role in roles:
                parts = []
                for cpu in item['cpu']:
                    parts.append(run([str(binaries[(role, item[role + '_package'])]),
                        '-test.run=^$', '-test.bench=' + item['benchmark'],
                        '-test.benchtime=' + args.benchtime, '-test.count=1',
                        '-test.benchmem', '-test.cpu=' + cpu],
                        cwd=roots[role], env=dict(os.environ, GOMAXPROCS=cpu)))
                result = ''.join(parts)
                if 'Benchmark' not in result or 'PASS' not in result:
                    raise RuntimeError(f'no successful benchmark rows for {group}/{role}')
                (output / f'{group}-{index + 1}-{role}.txt').write_text(result)
                with (output / f'{group}-{role}.txt').open('a') as combined:
                    combined.write(result)
                print(f'{group}: round {index + 1}/{args.rounds} {role}', flush=True)


if __name__ == '__main__':
    main()
