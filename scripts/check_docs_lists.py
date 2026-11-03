#!/usr/bin/env python3
"""Check that no list in the docs has more than three bullets.

A list of four items or more is numbered, so that one of them can be pointed
to ("item 4"), as the web app numbers its own lists (AGENTS.md). Three or
fewer stay bullets. Reads AGENTS.md, README.md and every Markdown file
under docs/, but not the finished plans in docs/plans/done/, which are a
record. Code blocks are
skipped.

Run from the repository root:  python3 scripts/check_docs_lists.py
Number them all:               python3 scripts/check_docs_lists.py --fix
"""

from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent

import re
import sys

BULLET = re.compile(r'^( *)[-*] (?=\S)')


def scan(lines):
    """The first bullet list of more than three items: (indent, [item start lines], end)."""
    fence = False
    i = 0
    n = len(lines)
    while i < n:
        line = lines[i]
        if line.strip().startswith('```') or line.strip().startswith('~~~'):
            fence = not fence
            i += 1
            continue
        if fence:
            i += 1
            continue
        m = BULLET.match(line)
        if not m:
            i += 1
            continue
        indent = len(m.group(1))
        items = [i]
        j = i + 1
        fence2 = False
        while j < n:
            l = lines[j]
            s = l.strip()
            if s.startswith('```') or s.startswith('~~~'):
                fence2 = not fence2
                j += 1
                continue
            if fence2:
                j += 1
                continue
            if s == '':
                # A blank line: the list goes on only if what follows is
                # inside it or another item of it.
                k = j + 1
                while k < n and lines[k].strip() == '':
                    k += 1
                if k >= n:
                    break
                nxt = lines[k]
                lead = len(nxt) - len(nxt.lstrip(' '))
                mm = BULLET.match(nxt)
                if (mm and len(mm.group(1)) == indent) or lead > indent:
                    j = k
                    continue
                break
            lead = len(l) - len(l.lstrip(' '))
            mm = BULLET.match(l)
            if mm and len(mm.group(1)) == indent:
                items.append(j)
                j += 1
                continue
            if lead > indent:
                j += 1
                continue
            # A line at the list's own indent that is not an item: a lazy
            # continuation of the item's text, unless it starts something.
            if lead == indent and not re.match(r'^ *(#|\||>|\d+\. |[-*] )', l) and lines[j - 1].strip() != '':
                j += 1
                continue
            break
        if len(items) > 3:
            return indent, items, j
        i = items[-1] + 1 if len(items) > 1 else i + 1
    return None


def number(lines, indent, items, end):
    out = list(lines)
    for k, start in enumerate(items):
        stop = items[k + 1] if k + 1 < len(items) else end
        marker = f'{k + 1}. '
        delta = len(marker) - 2
        line = out[start]
        out[start] = ' ' * indent + marker + line[indent + 2:]
        for x in range(start + 1, stop):
            if out[x].strip() and (len(out[x]) - len(out[x].lstrip(' '))) > indent:
                out[x] = ' ' * delta + out[x]
    return out


def convert(text):
    lines = text.split('\n')
    while True:
        found = scan(lines)
        if not found:
            return '\n'.join(lines)
        lines = number(lines, *found)




def files():
    docs = ROOT / "docs"
    found = sorted(p for p in docs.rglob("*.md") if "plans/done" not in p.as_posix())
    return [ROOT / "AGENTS.md", ROOT / "README.md"] + found


def main():
    fix = "--fix" in sys.argv[1:]
    found = []
    for path in files():
        text = path.read_text(encoding="utf-8")
        if fix:
            after = convert(text)
            if after != text:
                path.write_text(after, encoding="utf-8")
            continue
        lines = text.split("\n")
        while True:
            hit = scan(lines)
            if not hit:
                break
            indent, items, end = hit
            found.append(f"{path.relative_to(ROOT)}:{items[0] + 1}: a list of {len(items)} bullets; number it")
            lines = number(lines, indent, items, end)
    if fix:
        print("Every list of more than three items is numbered.")
        return 0
    if found:
        print("\n".join(found))
        print(f"\n{len(found)} lists of more than three bullets. Number them, or run: python3 scripts/check_docs_lists.py --fix")
        return 1
    print("No list in the docs has more than three bullets.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
