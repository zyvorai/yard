import type {ReactNode} from 'react';
import Link from '@docusaurus/Link';
import Layout from '@theme/Layout';
import Heading from '@theme/Heading';
import Reveal from '@site/src/components/Reveal';
import styles from './compare.module.css';

type Cell = boolean | string;

type Row = {
  label: string;
  core: Cell;
  enterprise: Cell;
};

const ROWS: Row[] = [
  {label: 'Asset registry, sites, and map', core: true, enterprise: true},
  {label: 'Telemetry, incidents, and work orders', core: true, enterprise: true},
  {label: 'Automations and webhooks', core: true, enterprise: true},
  {label: 'Severity policies and runbooks', core: true, enterprise: true},
  {label: 'HTTP ingest and the included simulator', core: true, enterprise: true},
  {label: 'Device Agent, Nodra, Fleet, and OTA connectors', core: 'Bring your own', enterprise: 'Available as products'},
  {label: 'Self-hosted (SQLite or Postgres)', core: true, enterprise: true},
  {label: 'Community support (GitHub issues)', core: true, enterprise: true},
  {label: 'Production support and SLAs', core: false, enterprise: true},
  {label: 'License', core: 'Apache-2.0', enterprise: 'Apache-2.0 core'},
  {label: 'Price', core: 'Free', enterprise: 'Contact sales'},
];

function Mark({value}: {value: Cell}) {
  if (typeof value === 'string') return <span className={styles.cellText}>{value}</span>;
  return value ? (
    <span className={styles.check} aria-label="Included">✓</span>
  ) : (
    <span className={styles.dash} aria-label="Not included">—</span>
  );
}

function CompareHeader() {
  return (
    <header className={styles.header}>
      <div className="container text--center">
        <Heading as="h1" className={styles.title}>
          Compare
        </Heading>
        <p className={styles.subtitle}>
          One codebase, Apache-2.0 core. Add support and optional connectors
          only if you need them.
        </p>
      </div>
    </header>
  );
}

function CompareTable() {
  return (
    <section>
      <div className="container">
        <Reveal className={styles.tableWrap}>
          <table className={styles.table}>
            <thead>
              <tr>
                <th></th>
                <th>
                  <div className={styles.planName}>Yard Core</div>
                  <div className={styles.planTag}>Open source, self-hosted</div>
                </th>
                <th>
                  <div className={styles.planName}>Zyvor Enterprise</div>
                  <div className={styles.planTag}>Core, plus support and add-ons</div>
                </th>
              </tr>
            </thead>
            <tbody>
              {ROWS.map((row) => (
                <tr key={row.label}>
                  <td className={styles.rowLabel}>{row.label}</td>
                  <td><Mark value={row.core} /></td>
                  <td><Mark value={row.enterprise} /></td>
                </tr>
              ))}
            </tbody>
          </table>
        </Reveal>
        <Reveal>
          <p className={styles.note}>
            There is one codebase. Zyvor Enterprise doesn&rsquo;t unlock
            hidden features — it adds a support contract, SLAs, and
            additional Zyvor products (Device Agent, Nodra, Fleet, OTA) for
            teams that want them managed. See{' '}
            <Link to="/docs/security">the security model</Link> and{' '}
            <Link to="/docs/guides/connectors">how connectors plug in</Link>.
          </p>
        </Reveal>
      </div>
    </section>
  );
}

function CompareCTA() {
  return (
    <section className={styles.cta}>
      <div className="container text--center">
        <Reveal>
          <Heading as="h2">Still deciding?</Heading>
          <p className={styles.ctaCopy}>
            Start with Core — it&rsquo;s free and runs alone. Talk to us
            later if you need support contracts, SLAs, or optional Zyvor
            connectors.
          </p>
          <div className={styles.ctaButtons}>
            <Link className="button button--primary button--lg" to="/docs/getting-started/quickstart">
              Get Started
            </Link>
            <Link className="button button--outline button--lg" to="mailto:sales@zyvor.dev">
              Contact sales@zyvor.dev
            </Link>
          </div>
        </Reveal>
      </div>
    </section>
  );
}

export default function Compare(): ReactNode {
  return (
    <Layout
      title="Compare"
      description="Compare Yard Core (Apache-2.0, self-hosted) with Zyvor Enterprise support and optional connectors.">
      <CompareHeader />
      <main>
        <CompareTable />
        <CompareCTA />
      </main>
    </Layout>
  );
}
