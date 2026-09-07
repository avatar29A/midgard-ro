import type { Rect } from "./contract";
export type Point = { x: number; y: number };
export function imagePoint(
  clientX: number,
  clientY: number,
  bounds: { left: number; top: number; width: number; height: number },
  width: number,
  height: number,
): Point {
  return {
    x: Math.max(
      0,
      Math.min(width, ((clientX - bounds.left) * width) / bounds.width),
    ),
    y: Math.max(
      0,
      Math.min(height, ((clientY - bounds.top) * height) / bounds.height),
    ),
  };
}
export function selectionRect(a: Point, b: Point): Rect | null {
  const x = Math.floor(Math.min(a.x, b.x)),
    y = Math.floor(Math.min(a.y, b.y));
  const right = Math.ceil(Math.max(a.x, b.x)),
    bottom = Math.ceil(Math.max(a.y, b.y));
  if (Math.abs(a.x - b.x) < 1 || Math.abs(a.y - b.y) < 1) return null;
  return { x, y, width: right - x, height: bottom - y };
}
export function assertRect(rect: Rect, width: number, height: number) {
  if (
    ![rect.x, rect.y, rect.width, rect.height].every(Number.isSafeInteger) ||
    rect.x < 0 ||
    rect.y < 0 ||
    rect.width < 1 ||
    rect.height < 1 ||
    rect.x + rect.width > width ||
    rect.y + rect.height > height
  )
    throw new Error("Выделение выходит за границы исходного изображения.");
}

export const resizeHandles = [
  "nw",
  "n",
  "ne",
  "e",
  "se",
  "s",
  "sw",
  "w",
] as const;
export type ResizeHandle = (typeof resizeHandles)[number];

/** Resize in source pixels, keeping the opposite edges fixed and a 1px minimum. */
export function resizeRect(
  rect: Rect,
  handle: ResizeHandle,
  delta: Point,
  width: number,
  height: number,
): Rect {
  let left = rect.x,
    top = rect.y,
    right = rect.x + rect.width,
    bottom = rect.y + rect.height;
  if (handle.includes("w"))
    left = Math.max(0, Math.min(right - 1, Math.round(rect.x + delta.x)));
  if (handle.includes("e"))
    right = Math.min(
      width,
      Math.max(left + 1, Math.round(rect.x + rect.width + delta.x)),
    );
  if (handle.includes("n"))
    top = Math.max(0, Math.min(bottom - 1, Math.round(rect.y + delta.y)));
  if (handle.includes("s"))
    bottom = Math.min(
      height,
      Math.max(top + 1, Math.round(rect.y + rect.height + delta.y)),
    );
  return { x: left, y: top, width: right - left, height: bottom - top };
}

export function moveRect(
  rect: Rect,
  delta: Point,
  width: number,
  height: number,
): Rect {
  return {
    ...rect,
    x: Math.max(0, Math.min(width - rect.width, Math.round(rect.x + delta.x))),
    y: Math.max(
      0,
      Math.min(height - rect.height, Math.round(rect.y + delta.y)),
    ),
  };
}
