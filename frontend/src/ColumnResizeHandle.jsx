import React, { useRef } from "react";

export function ColumnResizeHandle({ label, min, max, value, active, start, move, finish, step }) {
  const origin = useRef(null);
  return (
    <span
      className={active ? "column-handle dragging" : "column-handle"}
      role="separator"
      tabIndex={0}
      aria-orientation="vertical"
      aria-label={label}
      aria-valuemin={min}
      aria-valuemax={max}
      aria-valuenow={value}
      onPointerDown={(event) => {
        if (origin.current !== null || event.button !== 0 || start() === false) return;
        event.preventDefault();
        origin.current = event.clientX;
        event.currentTarget.setPointerCapture(event.pointerId);
      }}
      onPointerMove={(event) => {
        if (origin.current !== null && event.currentTarget.hasPointerCapture(event.pointerId))
          move(event.clientX - origin.current);
      }}
      onPointerUp={(event) => {
        if (origin.current === null || !event.currentTarget.hasPointerCapture(event.pointerId)) return;
        origin.current = null;
        finish(true);
      }}
      onLostPointerCapture={() => {
        if (origin.current === null) return;
        origin.current = null;
        finish(false);
      }}
      onKeyDown={(event) => {
        if (!["ArrowLeft", "ArrowRight"].includes(event.key)) return;
        event.preventDefault();
        step(event.key === "ArrowLeft" ? -1 : 1);
      }}
    />
  );
}
