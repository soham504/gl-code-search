"""Generate the README's SVG assets from result files (no plotting deps).

    python -m groundline.readme_assets

Writes docs/assets/{banner,results}-{light,dark}.svg and terminal.svg.
Colours: categorical slots 1-2 of the dataviz reference palette, validated
on GitHub's surfaces (#ffffff light, #0d1117 dark): all checks pass.
"""
from __future__ import annotations

import json
import subprocess
from html import escape
from pathlib import Path

from groundline.locsummary import LABEL, ORDER, RESULTS, hit

ROOT = Path(__file__).resolve().parents[2]
ASSETS = ROOT / "docs" / "assets"
SANS = "-apple-system, BlinkMacSystemFont, 'Segoe UI', Helvetica, Arial, sans-serif"
MONO = "ui-monospace, SFMono-Regular, Menlo, Consolas, monospace"

THEMES = {
    "light": {"text": "#1f2328", "muted": "#59636e", "grid": "#d1d9e0", "gl": "#2a78d6", "bm25": "#eb6834",
              "chip": "#f6f8fa", "chip_border": "#d1d9e0", "accent_bg": "#ddf4ff"},
    "dark": {"text": "#f0f6fc", "muted": "#9198a1", "grid": "#3d444d", "gl": "#3987e5", "bm25": "#d95926",
             "chip": "#151b23", "chip_border": "#3d444d", "accent_bg": "#0c2d6b"},
}


def text_w(s: str, size: float, mono: bool = False) -> float:
    """Rough rendered width; good enough to keep labels from colliding."""
    return len(s) * size * (0.6 if mono else 0.56)


def load_rows() -> list[tuple[str, int, float, float]]:
    rows = []
    for name in ORDER:
        if name == "lite":  # subset of SWE-bench full; shown in the table, not the chart
            continue
        p = RESULTS / f"loc_{name}.jsonl"
        if not p.exists():
            continue
        rs = [json.loads(line) for line in open(p)]
        n = len(rs)
        rows.append((LABEL[name], n, 100 * sum(hit(r["bm25_plain"], 1) for r in rs) / n,
                     100 * sum(hit(r["gl_full"], 1) for r in rs) / n))
    return rows


def results_svg(rows, t: dict) -> str:
    label_w = max(text_w(f"{lab} · n={n}", 13) for lab, n, _, _ in rows) + 16
    plot_x, plot_w, delta_w = label_w, 420, 70
    row_h, top = 34, 92
    width = int(plot_x + plot_w + delta_w + 16)
    height = top + row_h * len(rows) + 40
    x = lambda v: plot_x + plot_w * v / 100  # noqa: E731
    out = [f'<svg xmlns="http://www.w3.org/2000/svg" width="{width}" height="{height}" '
           f'viewBox="0 0 {width} {height}" font-family="{SANS}" role="img" '
           f'aria-label="Top-1 file localization accuracy, BM25 versus gl, per benchmark">',
           f'<text x="0" y="22" font-size="17" font-weight="600" fill="{t["text"]}">'
           f'Top-1 file localization accuracy</text>',
           f'<text x="0" y="44" font-size="13" fill="{t["muted"]}">Share of issues where the first-ranked file '
           f'is one the real fix changed. Higher is better.</text>']
    # legend (two series: always present)
    lx = 0
    for key, name in (("bm25", "Plain BM25"), ("gl", "gl (this repo)")):
        out.append(f'<circle cx="{lx + 6}" cy="68" r="5" fill="{t[key]}"/>')
        out.append(f'<text x="{lx + 16}" y="72" font-size="13" fill="{t["text"]}">{name}</text>')
        lx += 16 + text_w(name, 13) + 22
    # recessive hairline grid + ticks
    for v in (0, 25, 50, 75, 100):
        out.append(f'<line x1="{x(v):.1f}" y1="{top - 8}" x2="{x(v):.1f}" y2="{top + row_h * len(rows) - 10}" '
                   f'stroke="{t["grid"]}" stroke-width="1"/>')
        out.append(f'<text x="{x(v):.1f}" y="{top + row_h * len(rows) + 8}" font-size="12" '
                   f'fill="{t["muted"]}" text-anchor="middle">{v}%</text>')
    out.append(f'<text x="{plot_x + plot_w + delta_w}" y="{top - 14}" font-size="12" fill="{t["muted"]}" '
               f'text-anchor="end">gain</text>')
    for i, (lab, n, bm, gl) in enumerate(rows):
        cy = top + i * row_h + 8
        out.append(f'<text x="{plot_x - 16}" y="{cy + 4}" font-size="13" fill="{t["text"]}" text-anchor="end">'
                   f'{escape(lab)} <tspan fill="{t["muted"]}">· n={n}</tspan></text>')
        out.append(f'<line x1="{x(bm):.1f}" y1="{cy}" x2="{x(gl):.1f}" y2="{cy}" stroke="{t["grid"]}" '
                   f'stroke-width="2" stroke-linecap="round"/>')
        out.append(f'<circle cx="{x(bm):.1f}" cy="{cy}" r="5.5" fill="{t["bm25"]}"><title>{escape(lab)}: '
                   f'BM25 {bm:.1f}%</title></circle>')
        out.append(f'<circle cx="{x(gl):.1f}" cy="{cy}" r="5.5" fill="{t["gl"]}"><title>{escape(lab)}: '
                   f'gl {gl:.1f}%</title></circle>')
        out.append(f'<text x="{plot_x + plot_w + delta_w}" y="{cy + 4}" font-size="13" font-weight="600" '
                   f'fill="{t["text"]}" text-anchor="end">{gl - bm:+.1f} pts</text>')
    out.append("</svg>")
    return "\n".join(out)


def banner_svg(t: dict, verified_gain: float) -> str:
    w, h = 960, 212
    chips = [f"+{verified_gain:.0f} pts top-1 accuracy vs BM25", "0 LLM calls", "single Go binary · MCP · GitHub Action"]
    out = [f'<svg xmlns="http://www.w3.org/2000/svg" width="{w}" height="{h}" viewBox="0 0 {w} {h}" '
           f'font-family="{SANS}" role="img" aria-label="gl: find the file that needs fixing">',
           f'<rect x="0.5" y="0.5" width="{w - 1}" height="{h - 1}" rx="14" fill="{t["chip"]}" '
           f'stroke="{t["chip_border"]}"/>']
    # mark: a ranked list with the top hit highlighted
    for i, wd in enumerate((120, 92, 104)):
        y = 52 + i * 28
        fill = t["gl"] if i == 0 else t["grid"]
        out.append(f'<rect x="{w - 160}" y="{y}" width="{wd}" height="14" rx="4" fill="{fill}"/>')
        out.append(f'<text x="{w - 172}" y="{y + 12}" font-size="13" font-family="{MONO}" fill="{t["muted"]}" '
                   f'text-anchor="end">{i + 1}.</text>')
    out += [f'<text x="40" y="78" font-size="44" font-weight="700" font-family="{MONO}" fill="{t["text"]}">gl</text>',
            f'<text x="104" y="78" font-size="22" fill="{t["muted"]}">code-search</text>',
            f'<text x="40" y="118" font-size="20" fill="{t["text"]}">Paste a bug report. Get the files that need fixing.</text>']
    cx = 40
    for c in chips:
        cw = text_w(c, 13) + 24
        out.append(f'<rect x="{cx}" y="150" width="{cw:.0f}" height="30" rx="15" fill="{t["accent_bg"]}"/>')
        out.append(f'<text x="{cx + 12}" y="170" font-size="13" font-weight="600" fill="{t["text"]}">{escape(c)}</text>')
        cx += cw + 10
    out.append("</svg>")
    return "\n".join(out)


def terminal_svg(cmd: str, output: str) -> str:
    lines = [f"$ {cmd}"] + output.rstrip("\n").splitlines()
    lines = [ln if len(ln) <= 78 else ln[:77] + "…" for ln in lines]
    size, lh = 13, 20
    w = int(max(text_w(ln, size, mono=True) for ln in lines) + 48)
    h = 52 + lh * len(lines) + 16
    out = [f'<svg xmlns="http://www.w3.org/2000/svg" width="{w}" height="{h}" viewBox="0 0 {w} {h}" '
           f'font-family="{MONO}" font-size="{size}" role="img" aria-label="gl locate terminal output">',
           f'<rect width="{w}" height="{h}" rx="10" fill="#0d1117"/>',
           f'<rect width="{w}" height="34" rx="10" fill="#161b22"/><rect y="24" width="{w}" height="10" fill="#161b22"/>']
    for i, c in enumerate(("#ff5f57", "#febc2e", "#28c840")):
        out.append(f'<circle cx="{20 + i * 20}" cy="17" r="6" fill="{c}"/>')
    for i, ln in enumerate(lines):
        y = 58 + i * lh
        if i == 0:
            fill = "#7ee787"
        elif ln.startswith("#"):
            fill = "#9198a1"
        elif ln.lstrip().startswith("1."):
            fill = "#f0f6fc"
        else:
            fill = "#c9d1d9"
        weight = ' font-weight="700"' if ln.lstrip().startswith("1.") else ""
        out.append(f'<text x="20" y="{y}" fill="{fill}"{weight} xml:space="preserve">{escape(ln)}</text>')
    out.append("</svg>")
    return "\n".join(out)


def main() -> None:
    ASSETS.mkdir(parents=True, exist_ok=True)
    rows = load_rows()
    gains = {lab: gl - bm for lab, _, bm, gl in rows}
    verified_gain = gains.get(LABEL["verified"], 0.0)
    for mode, t in THEMES.items():
        (ASSETS / f"results-{mode}.svg").write_text(results_svg(rows, t))
        (ASSETS / f"banner-{mode}.svg").write_text(banner_svg(t, verified_gain))

    # A real run on a fresh (post-cutoff) beets issue whose fix touched beets/plugins.py and beets/events.py.
    issue = ROOT / "harness" / "issues" / "beetbox__beets-7036.txt"
    work = ROOT / "data" / "work" / "beetbox__beets-7036"
    if issue.exists() and work.exists():
        out = subprocess.run([str(ROOT / "bin" / "gl"), "-root", str(work), "-k", "5", "locate"],
                             input=issue.read_text(), capture_output=True, text=True, check=True).stdout
        out = out.replace(str(work), "beets")
        (ASSETS / "terminal.svg").write_text(terminal_svg("gl -k 5 locate < issue-7036.txt", out))
    print(f"wrote assets for {len(rows)} benchmarks to {ASSETS}")


if __name__ == "__main__":
    main()
