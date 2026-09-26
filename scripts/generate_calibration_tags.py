#!/usr/bin/env python3
"""Generate the printable calibration-target tags (tag36h11, IDs 100-104).

Dev-time tool: outputs are committed to ui/public/calibration-tags/ so the app
can host them. Requires python3 with opencv (cv2.aruco) and numpy.

    python3 scripts/generate_calibration_tags.py

Each tag36h11 marker is an 8x8 cell grid (6x6 data cells inside a 1-cell black
border). The black square is exactly TAG_SIZE_MM wide, so it prints at 150 mm.
A one-cell white quiet zone surrounds it. Keep TAG_SIZE_MM in sync with
position.TargetTagSize in the Go code.
"""

import re
import sys
from pathlib import Path

import cv2
import numpy as np

TAG_SIZE_MM = 150.0
GRID = 8
CELL_MM = TAG_SIZE_MM / GRID
QUIET_MM = CELL_MM
TOTAL_MM = TAG_SIZE_MM + 2 * QUIET_MM
TAG_IDS = (100, 101, 102, 103, 104)

OUT_DIR = Path(__file__).resolve().parent.parent / "ui" / "public" / "calibration-tags"


def fmt(value: float) -> str:
    return f"{value:.4f}".rstrip("0").rstrip(".")


def marker_cells(dictionary, tag_id: int) -> np.ndarray:
    """Return an 8x8 bool array, True = black cell."""
    img = cv2.aruco.generateImageMarker(dictionary, tag_id, GRID)
    return img == 0


def build_svg(cells: np.ndarray, tag_id: int) -> str:
    rects = []
    for row in range(GRID):
        for col in range(GRID):
            if cells[row, col]:
                x = QUIET_MM + col * CELL_MM
                y = QUIET_MM + row * CELL_MM
                rects.append(
                    f'    <rect x="{fmt(x)}" y="{fmt(y)}" width="{fmt(CELL_MM)}" height="{fmt(CELL_MM)}"/>'
                )
    total = fmt(TOTAL_MM)
    return (
        '<?xml version="1.0" encoding="UTF-8"?>\n'
        f'<svg xmlns="http://www.w3.org/2000/svg" width="{total}mm" height="{total}mm" '
        f'viewBox="0 0 {total} {total}">\n'
        f"  <title>tag36h11 ID {tag_id} ({fmt(TAG_SIZE_MM)} mm black square)</title>\n"
        f'  <rect width="{total}" height="{total}" fill="#fff"/>\n'
        '  <g fill="#000" shape-rendering="crispEdges">\n'
        + "\n".join(rects)
        + "\n  </g>\n</svg>\n"
    )


def parse_svg_cells(svg: str) -> np.ndarray:
    """Rebuild the 8x8 black-cell grid from the written SVG text."""
    cells = np.zeros((GRID, GRID), dtype=bool)
    pattern = re.compile(r'<rect x="([\d.]+)" y="([\d.]+)" width="([\d.]+)" height="([\d.]+)"/>')
    for x, y, w, h in pattern.findall(svg):
        assert abs(float(w) - CELL_MM) < 1e-3 and abs(float(h) - CELL_MM) < 1e-3
        col = round((float(x) - QUIET_MM) / CELL_MM)
        row = round((float(y) - QUIET_MM) / CELL_MM)
        assert abs(QUIET_MM + col * CELL_MM - float(x)) < 1e-3
        assert abs(QUIET_MM + row * CELL_MM - float(y)) < 1e-3
        cells[row, col] = True
    return cells


def verify(dictionary, tag_id: int, svg: str, expected_cells: np.ndarray) -> None:
    cells = parse_svg_cells(svg)
    assert np.array_equal(cells, expected_cells), f"tag {tag_id}: SVG cells differ from cv2 bits"

    scale = 20  # px per cell
    side = (GRID + 2) * scale
    canvas = np.full((side, side), 255, dtype=np.uint8)
    for row in range(GRID):
        for col in range(GRID):
            if cells[row, col]:
                y0, x0 = (row + 1) * scale, (col + 1) * scale
                canvas[y0 : y0 + scale, x0 : x0 + scale] = 0

    detector = cv2.aruco.ArucoDetector(dictionary, cv2.aruco.DetectorParameters())
    _, ids, _ = detector.detectMarkers(canvas)
    found = [] if ids is None else [int(i) for i in ids.flatten()]
    assert found == [tag_id], f"tag {tag_id}: detector found {found}"


def main() -> int:
    dictionary = cv2.aruco.getPredefinedDictionary(cv2.aruco.DICT_APRILTAG_36h11)
    OUT_DIR.mkdir(parents=True, exist_ok=True)

    for tag_id in TAG_IDS:
        cells = marker_cells(dictionary, tag_id)
        svg = build_svg(cells, tag_id)
        path = OUT_DIR / f"tag-{tag_id}.svg"
        path.write_text(svg, encoding="utf-8")
        verify(dictionary, tag_id, path.read_text(encoding="utf-8"), cells)
        print(f"wrote and verified {path.relative_to(OUT_DIR.parent.parent.parent)}")

    return 0


if __name__ == "__main__":
    sys.exit(main())
