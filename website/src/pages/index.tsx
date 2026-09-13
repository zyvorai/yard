import type {ReactNode} from 'react';
import clsx from 'clsx';
import Link from '@docusaurus/Link';
import useBaseUrl from '@docusaurus/useBaseUrl';
import Layout from '@theme/Layout';
import Heading from '@theme/Heading';
import FeatureHighlights from '@site/src/components/FeatureHighlights';
import ScreenshotStrip from '@site/src/components/ScreenshotStrip';
import Reveal from '@site/src/components/Reveal';

import styles from './index.module.css';

function HomepageHeader() {
  const heroShot = useBaseUrl('/00-overview.png');
  return (
    <header className={clsx('hero hero--primary', styles.heroBanner)}>
      <div className="container">
        <div className={styles.heroGrid}>
          <div>
            <Heading as="h1" className="hero__title">
              Register assets.
              <br />
              See health.
              <br />
              Close the work.
            </Heading>
            <p className="hero__subtitle">
              Open asset and operations platform — devices, sites, telemetry,
              incidents, and work orders. Zyvor connectors are optional. Yard
              runs alone.
            </p>
            <div className={styles.buttons}>
              <Link
                className="button button--secondary button--lg"
                to="/docs/getting-started/quickstart">
                Get Started
              </Link>
              <Link
                className="button button--outline button--lg button--secondary"
                to="https://github.com/zyvorai/yard">
                View on GitHub
              </Link>
            </div>
          </div>
          <div className={styles.heroMedia}>
            <img
              src={heroShot}
              alt="Yard Overview — asset health, incidents, and recent activity"
            />
            <p className={styles.heroMediaCaption}>
              Captured against a live lab deployment, not a mockup.
            </p>
          </div>
        </div>
      </div>
    </header>
  );
}

function ProblemStatement() {
  return (
    <section className={styles.problem}>
      <div className="container">
        <Reveal className="row">
          <div className="col col--8 col--offset-2 text--center">
            <Heading as="h2" className={styles.sectionHeading}>
              Why a standalone asset ops console?
            </Heading>
            <p>
              Logistics platforms optimize dispatch. Infrastructure platforms
              optimize runtimes. Field teams still stitch device health, site
              context, and maintenance work across three consoles.
            </p>
            <p>
              Yard’s core object is an <strong>asset</strong>. A device is one
              asset kind. Vehicles and route optimization can arrive later as
              an optional logistics extension — they do not own the data model.
              Device Agent, Nodra, Fleet, and OTA plug in when you need them.
            </p>
          </div>
        </Reveal>
      </div>
    </section>
  );
}

function TrustBand() {
  return (
    <section className={styles.trust}>
      <div className="container">
        <Reveal className={styles.trustGrid}>
          <div>
            <Heading as="h3" className={styles.sectionHeading}>
              Open, and ready to ship
            </Heading>
            <p>
              Apache-2.0. SQLite by default, Postgres when you need it. Real CI
              on every push (Go tests + console build). Sessions, hashed
              connector tokens, write RBAC, ingest rate limits, and{' '}
              <code>/metrics</code>.
            </p>
            <Link to="/docs/security">Read the security model →</Link>
          </div>
          <div className={styles.trustBadges}>
            <img
              src="https://github.com/zyvorai/yard/actions/workflows/ci.yml/badge.svg"
              alt="CI status"
            />
            <img
              src="https://img.shields.io/badge/License-Apache%202.0-blue.svg"
              alt="Apache 2.0 license"
            />
          </div>
        </Reveal>
      </div>
    </section>
  );
}

function EnterpriseCTA() {
  return (
    <section className={styles.enterprise}>
      <div className="container text--center">
        <Reveal>
          <Heading as="h2" className={styles.sectionHeading}>
            Need production support or SLAs?
          </Heading>
          <p className={styles.enterpriseCopy}>
            Yard’s core is Apache-2.0 and free to run. Zyvor Enterprise adds
            support contracts, SLAs, and additional products for teams that
            need them.
          </p>
          <Link
            className="button button--primary button--lg"
            to="mailto:sales@zyvor.dev">
            Contact sales@zyvor.dev
          </Link>
        </Reveal>
      </div>
    </section>
  );
}

export default function Home(): ReactNode {
  return (
    <Layout
      title="Yard — open asset and operations platform"
      description="Register assets, see health, and close the work. Standalone asset ops with optional Zyvor connectors.">
      <HomepageHeader />
      <main>
        <ProblemStatement />
        <Reveal>
          <FeatureHighlights />
        </Reveal>
        <Reveal>
          <ScreenshotStrip />
        </Reveal>
        <TrustBand />
        <EnterpriseCTA />
      </main>
    </Layout>
  );
}
