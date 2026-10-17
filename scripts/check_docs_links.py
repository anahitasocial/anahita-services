#!/usr/bin/env python3
"""Check that every link between the docs leads somewhere.

Reads every Markdown file git keeps (the docs, the plans, README.md,
AGENTS.md and the READMEs beside the code) and checks each relative link:
that the file or folder it names exists, and that a `#heading` after it is a
heading of that file, as GitHub makes them (lower case, punctuation dropped,
spaces to hyphens, `-1`, `-2` for a repeat) or an `<a id>` in it.

Links to the web (http, https, mailto) are not followed: that needs the
network, and they are the other repository's or the world's to keep. Code
blocks and `inline code` are skipped, so an example is never taken for a
link.

Run from the repository root:  python3 scripts/check_docs_links.py
"""

import re
import subprocess
import sys
from pathlib import Path
from urllib.parse import unquote

ROOT = Path(__file__).resolve().parent.parent


FENCE = re.compile(r"^\s*(```|~~~)")
HEADING = re.compile(r"^(#{1,6})\s+(.*?)\s*#*\s*$")
ANCHOR = re.compile(r"<a\s+(?:id|name)=\"([^\"]+)\"")
INLINE_CODE = re.compile(r"(`+)(.+?)\1")
# [text](target) and ![alt](target), with an optional "title" after the target.
LINK = re.compile(r"!?\[(?:[^\[\]]|\[[^\]]*\])*\]\(\s*<?([^)\s>]+)>?(?:\s+\"[^\"]*\")?\s*\)")
EXTERNAL = re.compile(r"^[a-z][a-z0-9+.-]*:", re.I)


def markdown_files():
    """The Markdown files git keeps or would keep: nothing ignored, so no
    builds, data or other people's code."""
    listed = subprocess.run(
        ["git", "ls-files", "--cached", "--others", "--exclude-standard", "*.md"],
        cwd=ROOT, capture_output=True, text=True, check=True,
    ).stdout.splitlines()
    for name in sorted(set(listed)):
        path = ROOT / name
        if path.exists():
            yield path


def lines_outside_code(path):
    """The file's lines with their numbers, leaving out fenced code blocks."""
    fenced = None
    for number, line in enumerate(path.read_text(encoding="utf-8").splitlines(), 1):
        fence = FENCE.match(line)
        if fence:
            if fenced is None:
                fenced = fence.group(1)
            elif fence.group(1) == fenced:
                fenced = None
            continue
        if fenced is None:
            yield number, line


def slug(heading):
    """A heading's anchor, the way GitHub makes it."""
    text = INLINE_CODE.sub(lambda m: m.group(2), heading)
    text = re.sub(r"!?\[([^\]]*)\]\([^)]*\)", r"\1", text)  # a link keeps its text
    text = re.sub(r"<[^>]+>", "", text)  # and tags go
    text = text.strip().lower()
    text = re.sub(r"[^\w\- ]", "", text)
    return text.replace(" ", "-")


_anchors = {}


def anchors(path):
    if path not in _anchors:
        found, seen = set(), {}
        for _, line in lines_outside_code(path):
            found.update(ANCHOR.findall(line))
            heading = HEADING.match(line)
            if heading:
                base = slug(heading.group(2))
                count = seen.get(base, 0)
                seen[base] = count + 1
                found.add(base if count == 0 else f"{base}-{count}")
        _anchors[path] = found
    return _anchors[path]


def broken_links(path):
    for number, line in lines_outside_code(path):
        for target in LINK.findall(INLINE_CODE.sub("", line)):
            if EXTERNAL.match(target):
                continue
            name, _, fragment = target.partition("#")
            name = unquote(name)
            if name.startswith("/"):
                dest = ROOT / name.lstrip("/")
            else:
                dest = (path.parent / name).resolve() if name else path
            if not dest.exists():
                yield number, target, "no such file"
            elif fragment and dest.suffix == ".md" and unquote(fragment).lower() not in anchors(dest):
                yield number, target, "no such heading"


def main():
    problems = 0
    files = 0
    for path in markdown_files():
        files += 1
        for number, target, why in broken_links(path):
            problems += 1
            print(f"{path.relative_to(ROOT)}:{number}: {target}: {why}")
    if problems:
        print(f"\n{problems} broken link{'s' if problems != 1 else ''} in {files} files.")
        return 1
    print(f"Every link between the docs leads somewhere ({files} files).")
    return 0


if __name__ == "__main__":
    sys.exit(main())
