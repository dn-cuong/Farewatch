import { useEffect, useMemo, useRef, useState } from 'react';
import styles from './CalendarPicker.module.css';

type Props = {
  id: string;
  value: string;
  onChange: (value: string) => void;
  min?: string;
  disabled?: boolean;
  placeholder?: string;
};

const weekdays = ['Su', 'Mo', 'Tu', 'We', 'Th', 'Fr', 'Sa'];

function parseDate(value?: string) {
  if (!value) return null;
  const [year, month, day] = value.split('-').map(Number);
  if (!year || !month || !day) return null;
  return new Date(year, month - 1, day);
}

function toValue(date: Date) {
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, '0');
  const day = String(date.getDate()).padStart(2, '0');
  return `${year}-${month}-${day}`;
}

function sameDay(a: Date | null, b: Date) {
  return Boolean(
    a &&
      a.getFullYear() === b.getFullYear() &&
      a.getMonth() === b.getMonth() &&
      a.getDate() === b.getDate(),
  );
}

export function CalendarPicker({
  id,
  value,
  onChange,
  min,
  disabled = false,
  placeholder = 'Select date',
}: Props) {
  const rootRef = useRef<HTMLDivElement>(null);
  const selected = parseDate(value);
  const minimum = parseDate(min);
  const [open, setOpen] = useState(false);
  const [dropUp, setDropUp] = useState(false);
  const [month, setMonth] = useState(() => {
    const initial = selected ?? minimum ?? new Date();
    return new Date(initial.getFullYear(), initial.getMonth(), 1);
  });

  useEffect(() => {
    function close(event: PointerEvent) {
      if (!rootRef.current?.contains(event.target as Node)) setOpen(false);
    }
    document.addEventListener('pointerdown', close);
    return () => document.removeEventListener('pointerdown', close);
  }, []);

  useEffect(() => {
    if (!open) return;
    const initial = selected ?? minimum ?? new Date();
    setMonth(new Date(initial.getFullYear(), initial.getMonth(), 1));
    const rect = rootRef.current?.getBoundingClientRect();
    if (!rect) return;
    const panelHeight = 350;
    setDropUp(
      rect.bottom + panelHeight > window.innerHeight && rect.top > panelHeight,
    );
  }, [open, value, min]);

  const days = useMemo(() => {
    const first = new Date(month.getFullYear(), month.getMonth(), 1);
    const start = new Date(first);
    start.setDate(1 - first.getDay());
    return Array.from({ length: 42 }, (_, index) => {
      const date = new Date(start);
      date.setDate(start.getDate() + index);
      return date;
    });
  }, [month]);

  const label = selected
    ? selected.toLocaleDateString(undefined, { month: 'short', day: 'numeric', year: 'numeric' })
    : placeholder;
  const today = new Date();
  today.setHours(0, 0, 0, 0);

  function choose(date: Date) {
    onChange(toValue(date));
    setOpen(false);
    requestAnimationFrame(() => document.getElementById(id)?.focus());
  }

  return (
    <div className={styles.root} ref={rootRef}>
      <button
        id={id}
        className={`${styles.trigger} ${open ? styles.triggerOpen : ''}`}
        type="button"
        aria-haspopup="dialog"
        aria-expanded={open}
        disabled={disabled}
        onClick={() => setOpen((current) => !current)}
        onKeyDown={(event) => {
          if (event.key === 'Escape') setOpen(false);
        }}
      >
        <span className={!selected ? styles.placeholder : undefined}>{label}</span>
        <svg viewBox="0 0 20 20" aria-hidden="true">
          <rect x="3" y="4.5" width="14" height="12.5" rx="2" />
          <path d="M6.5 2.5v4M13.5 2.5v4M3 8.5h14" />
        </svg>
      </button>

      {open && (
        <div
          className={`${styles.popover} ${dropUp ? styles.popoverUp : ''}`}
          role="dialog"
          aria-label="Choose a date"
        >
          <div className={styles.header}>
            <button
              type="button"
              aria-label="Previous month"
              onClick={() => setMonth(new Date(month.getFullYear(), month.getMonth() - 1, 1))}
            >
              ‹
            </button>
            <strong>{month.toLocaleDateString(undefined, { month: 'long', year: 'numeric' })}</strong>
            <button
              type="button"
              aria-label="Next month"
              onClick={() => setMonth(new Date(month.getFullYear(), month.getMonth() + 1, 1))}
            >
              ›
            </button>
          </div>
          <div className={styles.grid}>
            {weekdays.map((day) => (
              <span className={styles.weekday} key={day}>{day}</span>
            ))}
            {days.map((date) => {
              const dateValue = toValue(date);
              const unavailable = Boolean(minimum && date < minimum);
              const outside = date.getMonth() !== month.getMonth();
              return (
                <button
                  type="button"
                  key={dateValue}
                  className={[
                    styles.day,
                    outside ? styles.outside : '',
                    sameDay(selected, date) ? styles.selected : '',
                    sameDay(today, date) ? styles.today : '',
                  ].join(' ')}
                  disabled={unavailable}
                  aria-label={date.toLocaleDateString()}
                  aria-pressed={sameDay(selected, date)}
                  onClick={() => choose(date)}
                >
                  {date.getDate()}
                </button>
              );
            })}
          </div>
        </div>
      )}
    </div>
  );
}
