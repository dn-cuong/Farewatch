import { useEffect, useState } from 'react';
import styles from './FlipNumber.module.css';

type Props = {
  value: number;
  prefix?: string;
  className?: string;
};

function toDigits(value: number) {
  return Math.max(0, Math.round(value)).toString().split('');
}

export function FlipNumber({ value, prefix = '$', className }: Props) {
  const [digits, setDigits] = useState(() => toDigits(value));

  useEffect(() => {
    setDigits(toDigits(value));
  }, [value]);

  return (
    <span className={[styles.root, className].filter(Boolean).join(' ')} aria-label={`${prefix}${Math.round(value)}`}>
      {prefix ? <span className={styles.prefix}>{prefix}</span> : null}
      <span className={styles.digits}>
        {digits.map((d, i) => (
          <span className={styles.digit} key={`${i}-${digits.length}`}>
            <span className={styles.stack} style={{ ['--offset' as string]: d }}>
              {Array.from({ length: 10 }, (_, n) => (
                <span key={n}>{n}</span>
              ))}
            </span>
          </span>
        ))}
      </span>
    </span>
  );
}
