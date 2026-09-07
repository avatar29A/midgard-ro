import { describe, expect, it } from "vitest";
import { assertRect, imagePoint, selectionRect } from "./geometry";
describe("source pixel coordinates", () => {
  it("handles fit, zoom, scrolled origins and reverse drags without a DPR multiplier", () => {
    for (const scale of [0.25, 0.5, 1, 2, 4]) {
      const bounds = {
        left: -73,
        top: 121,
        width: 800 * scale,
        height: 600 * scale,
      };
      const a = imagePoint(
        bounds.left + 350 * scale,
        bounds.top + 400 * scale,
        bounds,
        800,
        600,
      );
      const b = imagePoint(
        bounds.left + 120 * scale,
        bounds.top + 80 * scale,
        bounds,
        800,
        600,
      );
      expect(selectionRect(a, b)).toEqual({
        x: 120,
        y: 80,
        width: 230,
        height: 320,
      });
    }
  });
  it("clips drag to image edges and rejects clicks and invalid regions", () => {
    expect(
      imagePoint(
        -10,
        999,
        { left: 0, top: 0, width: 100, height: 50 },
        200,
        100,
      ),
    ).toEqual({ x: 0, y: 100 });
    expect(selectionRect({ x: 10, y: 10 }, { x: 10.4, y: 10.4 })).toBeNull();
    expect(() =>
      assertRect({ x: 190, y: 0, width: 11, height: 1 }, 200, 100),
    ).toThrow();
    expect(() =>
      assertRect({ x: 0, y: 0, width: 0, height: 1 }, 200, 100),
    ).toThrow();
  });
});

import { resizeHandles, resizeRect, moveRect } from "./geometry";
it("resizes all eight handles with fixed opposite edges and source bounds", () => {
  const original = { x: 20, y: 30, width: 40, height: 50 };
  const expected = {
    nw: { x: 25, y: 40, width: 35, height: 40 },
    n: { x: 20, y: 40, width: 40, height: 40 },
    ne: { x: 20, y: 40, width: 45, height: 40 },
    e: { x: 20, y: 30, width: 45, height: 50 },
    se: { x: 20, y: 30, width: 45, height: 60 },
    s: { x: 20, y: 30, width: 40, height: 60 },
    sw: { x: 25, y: 30, width: 35, height: 60 },
    w: { x: 25, y: 30, width: 35, height: 50 },
  };
  for (const h of resizeHandles)
    expect(resizeRect(original, h, { x: 5, y: 10 }, 200, 150)).toEqual(
      expected[h],
    );
  expect(resizeRect(original, "nw", { x: 999, y: 999 }, 200, 150)).toEqual({
    x: 59,
    y: 79,
    width: 1,
    height: 1,
  });
  expect(resizeRect(original, "se", { x: 999, y: 999 }, 200, 150)).toEqual({
    x: 20,
    y: 30,
    width: 180,
    height: 120,
  });
  expect(resizeRect(original, "nw", { x: -999, y: -999 }, 200, 150)).toEqual({
    x: 0,
    y: 0,
    width: 60,
    height: 80,
  });
  expect(original).toEqual({ x: 20, y: 30, width: 40, height: 50 });
});
it("moves a resized box without changing its dimensions or leaving the image", () => {
  const resized = resizeRect(
    { x: 20, y: 20, width: 40, height: 30 },
    "se",
    { x: 20, y: 10 },
    200,
    100,
  );
  expect(moveRect(resized, { x: 10, y: 10 }, 200, 100)).toEqual({
    x: 30,
    y: 30,
    width: 60,
    height: 40,
  });
  expect(moveRect(resized, { x: 999, y: -999 }, 200, 100)).toEqual({
    x: 140,
    y: 0,
    width: 60,
    height: 40,
  });
});
