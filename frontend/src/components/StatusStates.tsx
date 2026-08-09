import type { ReactNode } from 'react';
import styles from './StatusStates.module.css';

type Props = {
  title: string;
  message: string;
  action?: ReactNode;
};

export function LoadingState({ title = 'Loading', message = 'Please wait…' }: Partial<Props>) {
  return (
    <div className={styles.state} role="status">
      <div className={styles.bars} aria-hidden>
        <i /><i /><i /><i /><i />
      </div>
      <div className={styles.title}>{title}</div>
      <p className={styles.copy}>{message}</p>
    </div>
  );
}

export function EmptyState({ title, message, action }: Props) {
  return (
    <div className={styles.state}>
      <div className={styles.mark}>-</div>
      <div className={styles.title}>{title}</div>
      <p className={styles.copy}>{message}</p>
      {action}
    </div>
  );
}

export function ErrorState({ title, message, action }: Props) {
  return (
    <div className={`${styles.state} ${styles.error}`} role="alert">
      <div className={styles.mark}>!</div>
      <div className={styles.title}>{title}</div>
      <p className={styles.copy}>{message}</p>
      {action}
    </div>
  );
}
