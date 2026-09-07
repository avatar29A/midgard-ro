import { useEffect, useState, type RefObject } from "react";

/** Temporary hand tool scoped to one image viewport; never consumes text input. */
export function useImagePan(
  view: RefObject<HTMLDivElement | null>,
  ready: boolean,
) {
  const [spaceHeld, setSpaceHeld] = useState(false);
  const [panning, setPanning] = useState(false);
  useEffect(() => {
    const el = view.current;
    if (!el || !ready) return;
    let hovered = false,
      space = false;
    let drag: {
      pointerId: number;
      x: number;
      y: number;
      left: number;
      top: number;
    } | null = null;
    const enter = () => {
      hovered = true;
    };
    const leave = () => {
      hovered = false;
    };
    const keydown = (e: KeyboardEvent) => {
      if (e.code !== "Space" || e.altKey || e.ctrlKey || e.metaKey) return;
      if (
        e.target instanceof Element &&
        e.target.closest(
          "input, textarea, select, button, [contenteditable]:not([contenteditable='false']), [role='textbox']",
        )
      )
        return;
      if (!hovered && !el.contains(document.activeElement)) return;
      e.preventDefault();
      space = true;
      setSpaceHeld(true);
    };
    const keyup = (e: KeyboardEvent) => {
      if (e.code === "Space") {
        space = false;
        setSpaceHeld(false);
      }
    };
    const stop = () => {
      const old = drag;
      drag = null;
      setPanning(false);
      if (old && el.hasPointerCapture(old.pointerId))
        el.releasePointerCapture(old.pointerId);
    };
    const down = (e: PointerEvent) => {
      if (drag || !(e.button === 1 || (e.button === 0 && space))) return;
      e.preventDefault();
      el.focus({ preventScroll: true });
      el.setPointerCapture(e.pointerId);
      drag = {
        pointerId: e.pointerId,
        x: e.clientX,
        y: e.clientY,
        left: el.scrollLeft,
        top: el.scrollTop,
      };
      setPanning(true);
    };
    const move = (e: PointerEvent) => {
      if (!drag || drag.pointerId !== e.pointerId) return;
      e.preventDefault();
      el.scrollLeft = drag.left - (e.clientX - drag.x);
      el.scrollTop = drag.top - (e.clientY - drag.y);
    };
    const up = (e: PointerEvent) => {
      if (drag?.pointerId === e.pointerId) {
        move(e);
        stop();
      }
    };
    const cancel = (e: PointerEvent) => {
      if (drag?.pointerId === e.pointerId) stop();
    };
    const blur = () => {
      space = false;
      setSpaceHeld(false);
      stop();
    };
    const aux = (e: MouseEvent) => {
      if (e.button === 1) e.preventDefault();
    };
    el.addEventListener("pointerenter", enter);
    el.addEventListener("pointerleave", leave);
    el.addEventListener("pointerdown", down);
    el.addEventListener("pointermove", move);
    el.addEventListener("pointerup", up);
    el.addEventListener("pointercancel", cancel);
    el.addEventListener("lostpointercapture", cancel);
    el.addEventListener("auxclick", aux);
    window.addEventListener("keydown", keydown);
    window.addEventListener("keyup", keyup);
    window.addEventListener("blur", blur);
    return () => {
      el.removeEventListener("pointerenter", enter);
      el.removeEventListener("pointerleave", leave);
      el.removeEventListener("pointerdown", down);
      el.removeEventListener("pointermove", move);
      el.removeEventListener("pointerup", up);
      el.removeEventListener("pointercancel", cancel);
      el.removeEventListener("lostpointercapture", cancel);
      el.removeEventListener("auxclick", aux);
      window.removeEventListener("keydown", keydown);
      window.removeEventListener("keyup", keyup);
      window.removeEventListener("blur", blur);
      blur();
    };
  }, [view, ready]);
  return { spaceHeld, panning };
}
