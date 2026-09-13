import type {ReactNode} from 'react';
import Link from '@docusaurus/Link';
import styles from './styles.module.css';

export type RelatedArticle = {
  label: string;
  to: string;
  description?: string;
};

/**
 * Apple-Support-style "Related articles" footer for a doc page: bordered
 * tiles rather than a plain bullet list. Opt in per-page via MDX import.
 */
export default function RelatedArticles({items}: {items: RelatedArticle[]}): ReactNode {
  return (
    <div className={styles.wrap}>
      <p className={styles.heading}>Related articles</p>
      <div className={styles.grid}>
        {items.map((item) => (
          <Link key={item.to} to={item.to} className={styles.tile}>
            <span className={styles.tileLabel}>{item.label}</span>
            {item.description && <span className={styles.tileDescription}>{item.description}</span>}
            <span className={styles.tileArrow}>→</span>
          </Link>
        ))}
      </div>
    </div>
  );
}
