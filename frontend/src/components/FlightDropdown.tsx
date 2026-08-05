import {
  useEffect,
  useId,
  useMemo,
  useRef,
  useState,
  type KeyboardEvent,
} from 'react';
import styles from './FlightDropdown.module.css';

export type FlightDropdownOption = {
  value: string;
  label: string;
  code?: string;
  group?: string;
  keywords?: string;
};

type Props = {
  id: string;
  value: string;
  options: FlightDropdownOption[];
  onChange: (value: string) => void;
  placeholder?: string;
  searchPlaceholder?: string;
  searchable?: boolean;
  disabled?: boolean;
};

export function FlightDropdown({
  id,
  value,
  options,
  onChange,
  placeholder = 'Select',
  searchPlaceholder = 'Search…',
  searchable = false,
  disabled = false,
}: Props) {
  const generatedId = useId();
  const listboxId = `${id}-${generatedId}-listbox`;
  const rootRef = useRef<HTMLDivElement>(null);
  const searchRef = useRef<HTMLInputElement>(null);
  const menuRef = useRef<HTMLDivElement>(null);
  const [open, setOpen] = useState(false);
  const [dropUp, setDropUp] = useState(false);
  const [query, setQuery] = useState('');
  const [activeIndex, setActiveIndex] = useState(0);

  const selected = options.find((option) => option.value === value);
  const filtered = useMemo(() => {
    const normalized = query.trim().toLowerCase();
    if (!normalized) return options;
    return options.filter((option) =>
      [option.code, option.label, option.group, option.keywords]
        .filter(Boolean)
        .join(' ')
        .toLowerCase()
        .includes(normalized),
    );
  }, [options, query]);

  useEffect(() => {
    function closeOnOutsideClick(event: PointerEvent) {
      if (!rootRef.current?.contains(event.target as Node)) {
        setOpen(false);
      }
    }
    document.addEventListener('pointerdown', closeOnOutsideClick);
    return () => document.removeEventListener('pointerdown', closeOnOutsideClick);
  }, []);

  useEffect(() => {
    if (!open) {
      setQuery('');
      return;
    }
    const selectedIndex = Math.max(
      0,
      filtered.findIndex((option) => option.value === value),
    );
    setActiveIndex(selectedIndex);
    const rect = rootRef.current?.getBoundingClientRect();
    if (rect) {
      const panelHeight = 360;
      setDropUp(rect.bottom + panelHeight > window.innerHeight && rect.top > panelHeight);
    }
    requestAnimationFrame(() => {
      if (searchable) searchRef.current?.focus();
      else menuRef.current?.focus();
    });
  }, [open, searchable, value]);

  useEffect(() => {
    if (activeIndex >= filtered.length) setActiveIndex(Math.max(0, filtered.length - 1));
  }, [activeIndex, filtered.length]);

  function choose(option: FlightDropdownOption) {
    onChange(option.value);
    setOpen(false);
  }

  function handleKeyboard(event: KeyboardEvent<HTMLElement>) {
    if (event.key === 'Escape') {
      event.preventDefault();
      setOpen(false);
      document.getElementById(id)?.focus();
      return;
    }
    if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
      event.preventDefault();
      if (!open) {
        setOpen(true);
        return;
      }
      const direction = event.key === 'ArrowDown' ? 1 : -1;
      setActiveIndex((current) => {
        const next = current + direction;
        if (next < 0) return Math.max(0, filtered.length - 1);
        if (next >= filtered.length) return 0;
        return next;
      });
      return;
    }
    if (event.key === 'Enter' && open && filtered[activeIndex]) {
      event.preventDefault();
      choose(filtered[activeIndex]);
    }
  }

  let lastGroup = '';

  return (
    <div className={styles.root} ref={rootRef}>
      <button
        id={id}
        className={`${styles.trigger} ${open ? styles.triggerOpen : ''}`}
        type="button"
        role="combobox"
        aria-expanded={open}
        aria-controls={listboxId}
        aria-haspopup="listbox"
        disabled={disabled}
        onClick={() => setOpen((current) => !current)}
        onKeyDown={handleKeyboard}
      >
        <span className={styles.triggerText}>
          {selected?.code && <strong className={styles.selectedCode}>{selected.code}</strong>}
          <span className={selected ? styles.selectedLabel : styles.placeholder}>
            {selected?.label ?? placeholder}
          </span>
        </span>
        <svg className={styles.chevron} viewBox="0 0 16 16" aria-hidden="true">
          <path d="m4 6 4 4 4-4" />
        </svg>
      </button>

      {open && (
        <div className={`${styles.popover} ${dropUp ? styles.popoverUp : ''}`}>
          {searchable && (
            <div className={styles.searchWrap}>
              <svg viewBox="0 0 20 20" aria-hidden="true">
                <circle cx="9" cy="9" r="5.5" />
                <path d="m13 13 4 4" />
              </svg>
              <input
                ref={searchRef}
                className={styles.searchInput}
                value={query}
                onChange={(event) => {
                  setQuery(event.target.value);
                  setActiveIndex(0);
                }}
                onKeyDown={handleKeyboard}
                placeholder={searchPlaceholder}
                aria-label={searchPlaceholder}
              />
              {query && (
                <button type="button" onClick={() => setQuery('')} aria-label="Clear search">
                  ×
                </button>
              )}
            </div>
          )}

          <div
            id={listboxId}
            className={styles.menu}
            ref={menuRef}
            role="listbox"
            tabIndex={-1}
            aria-activedescendant={
              filtered[activeIndex] ? `${listboxId}-${filtered[activeIndex].value}` : undefined
            }
            onKeyDown={handleKeyboard}
          >
            {filtered.length === 0 && (
              <div className={styles.empty}>No matching airports</div>
            )}
            {filtered.map((option, index) => {
              const showGroup = Boolean(option.group && option.group !== lastGroup);
              if (option.group) lastGroup = option.group;
              const isSelected = option.value === value;
              const isActive = index === activeIndex;
              return (
                <div key={option.value}>
                  {showGroup && <div className={styles.group}>{option.group}</div>}
                  <button
                    id={`${listboxId}-${option.value}`}
                    className={`${styles.option} ${isActive ? styles.optionActive : ''}`}
                    type="button"
                    role="option"
                    aria-selected={isSelected}
                    onMouseEnter={() => setActiveIndex(index)}
                    onClick={() => choose(option)}
                  >
                    <span className={styles.optionContent}>
                      {option.code && <strong>{option.code}</strong>}
                      <span>{option.label}</span>
                    </span>
                    {isSelected && (
                      <svg className={styles.check} viewBox="0 0 16 16" aria-hidden="true">
                        <path d="m3 8 3 3 7-7" />
                      </svg>
                    )}
                  </button>
                </div>
              );
            })}
          </div>
        </div>
      )}
    </div>
  );
}
