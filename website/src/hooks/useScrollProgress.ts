import {useEffect, useRef} from 'react';

/**
 * Tracks how far an element has scrolled past the top of the viewport (0 at
 * the top, 1 once it's scrolled a full viewport height past it) and writes
 * it onto a CSS custom property on that element via rAF-throttled
 * scroll/resize listeners. CSS then reads the property with a `, 0`
 * fallback, so consumers work whether or not this hook ever set it.
 *
 * Skipped entirely under prefers-reduced-motion — the property is simply
 * never written, and reduced-motion CSS should ignore it anyway.
 */
export function useScrollProgress<T extends HTMLElement>(property = '--zv-progress') {
  const ref = useRef<T>(null);

  useEffect(() => {
    const node = ref.current;
    if (!node || typeof window === 'undefined') return;
    if (window.matchMedia('(prefers-reduced-motion: reduce)').matches) return;

    let ticking = false;
    const update = () => {
      ticking = false;
      const rect = node.getBoundingClientRect();
      const viewport = window.innerHeight || 1;
      const progress = Math.min(Math.max(-rect.top / viewport, 0), 1);
      node.style.setProperty(property, progress.toFixed(4));
    };
    const onScroll = () => {
      if (!ticking) {
        ticking = true;
        requestAnimationFrame(update);
      }
    };

    update();
    window.addEventListener('scroll', onScroll, {passive: true});
    window.addEventListener('resize', onScroll);
    return () => {
      window.removeEventListener('scroll', onScroll);
      window.removeEventListener('resize', onScroll);
    };
  }, [property]);

  return ref;
}
