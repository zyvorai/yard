import {useEffect, useRef, useState} from 'react';
import type {ReactNode} from 'react';
import clsx from 'clsx';
import Link from '@docusaurus/Link';
import useBaseUrl from '@docusaurus/useBaseUrl';
import Heading from '@theme/Heading';
import Reveal from '@site/src/components/Reveal';
import {FEATURES, type FeatureEntry} from '@site/src/data/features';
import styles from './styles.module.css';

type StepProps = {
  feature: FeatureEntry;
  index: number;
  active: boolean;
  onActivate: (index: number) => void;
};

function StoryStep({feature, index, active, onActivate}: StepProps) {
  const ref = useRef<HTMLDivElement>(null);
  const mobileShot = useBaseUrl(feature.image);

  useEffect(() => {
    const node = ref.current;
    if (!node || typeof IntersectionObserver === 'undefined') return;
    const observer = new IntersectionObserver(
      ([entry]) => {
        if (entry.isIntersecting) onActivate(index);
      },
      {threshold: 0.5, rootMargin: '-20% 0px -20% 0px'},
    );
    observer.observe(node);
    return () => observer.disconnect();
  }, [index, onActivate]);

  return (
    <div ref={ref} className={clsx(styles.step, active && styles.stepActive)}>
      <span className={styles.stepIndex}>{String(index + 1).padStart(2, '0')}</span>
      <Heading as="h3">{feature.title}</Heading>
      <p>{feature.description}</p>
      <Reveal className={styles.mobileFrame}>
        <img src={mobileShot} alt={feature.caption} loading="lazy" />
      </Reveal>
      <Link to={feature.to} className={styles.stepLink}>
        Learn more →
      </Link>
    </div>
  );
}

function StoryImage({feature, active}: {feature: FeatureEntry; active: boolean}) {
  const src = useBaseUrl(feature.image);
  return (
    <div className={clsx(styles.frame, active && styles.frameActive)}>
      <img src={src} alt={feature.caption} loading="lazy" />
    </div>
  );
}

export default function FeatureScrollStory(): ReactNode {
  const [activeIndex, setActiveIndex] = useState(0);

  return (
    <section className={styles.story}>
      <div className="container">
        <Reveal>
          <Heading as="h2" className={styles.heading}>
            One console. Every asset.
          </Heading>
        </Reveal>
        <div className={styles.layout}>
          <div className={styles.sticky}>
            <div className={styles.stickyInner}>
              {FEATURES.map((feature, index) => (
                <StoryImage key={feature.image} feature={feature} active={index === activeIndex} />
              ))}
            </div>
            <p className={styles.caption}>{FEATURES[activeIndex].caption}</p>
          </div>
          <div className={styles.steps}>
            {FEATURES.map((feature, index) => (
              <StoryStep
                key={feature.title}
                feature={feature}
                index={index}
                active={index === activeIndex}
                onActivate={setActiveIndex}
              />
            ))}
          </div>
        </div>
      </div>
    </section>
  );
}
