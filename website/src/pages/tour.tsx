import type {ReactNode} from 'react';
import clsx from 'clsx';
import Link from '@docusaurus/Link';
import useBaseUrl from '@docusaurus/useBaseUrl';
import Layout from '@theme/Layout';
import Heading from '@theme/Heading';
import Reveal from '@site/src/components/Reveal';
import {FEATURES, type FeatureEntry} from '@site/src/data/features';
import styles from './tour.module.css';

function TourHeader() {
  return (
    <header className={styles.header}>
      <div className="container text--center">
        <Heading as="h1" className={styles.title}>
          Product tour
        </Heading>
        <p className={styles.subtitle}>
          Every screenshot below is captured against a real, running lab
          deployment — not a mockup.
        </p>
      </div>
    </header>
  );
}

function FeatureBlock({feature, index}: {feature: FeatureEntry; index: number}) {
  const src = useBaseUrl(feature.image);
  return (
    <Reveal className={clsx(styles.block, index % 2 === 1 && styles.reverse)}>
      <div className={styles.blockMedia}>
        <img src={src} alt={feature.caption} loading="lazy" />
      </div>
      <div className={styles.blockText}>
        <Heading as="h2">{feature.title}</Heading>
        <p>{feature.description}</p>
        <p className={styles.blockCaption}>{feature.caption}</p>
        <Link to={feature.to} className={styles.blockLink}>
          Learn more →
        </Link>
      </div>
    </Reveal>
  );
}

function StatCallout({children}: {children: ReactNode}) {
  return (
    <Reveal className={styles.statCallout}>
      <p>{children}</p>
    </Reveal>
  );
}

function TourCTA() {
  return (
    <section className={styles.cta}>
      <div className="container text--center">
        <Reveal>
          <Heading as="h2">See it running in a minute</Heading>
          <p className={styles.ctaCopy}>
            Yard&rsquo;s core is Apache-2.0. Clone it, run it, and register
            your first asset without touching a connector.
          </p>
          <div className={styles.ctaButtons}>
            <Link className="button button--primary button--lg" to="/docs/getting-started/quickstart">
              Get Started
            </Link>
            <Link className="button button--outline button--lg" to="https://github.com/zyvorai/yard">
              View on GitHub
            </Link>
          </div>
          <p style={{marginTop: '1.5rem'}}>
            <Link to="/compare">Compare Core vs Enterprise →</Link>
          </p>
        </Reveal>
      </div>
    </section>
  );
}

export default function Tour(): ReactNode {
  return (
    <Layout
      title="Product tour"
      description="A walkthrough of the Yard console, captured against a live lab deployment.">
      <TourHeader />
      <main>
        <div className="container">
          {FEATURES.map((feature, index) => (
            <div key={feature.title}>
              <FeatureBlock feature={feature} index={index} />
              {index === 1 && (
                <StatCallout>1 asset model. Every device, vehicle, or sensor fits it.</StatCallout>
              )}
              {index === 3 && <StatCallout>4 optional connectors, 0 required.</StatCallout>}
            </div>
          ))}
        </div>
        <TourCTA />
      </main>
    </Layout>
  );
}
