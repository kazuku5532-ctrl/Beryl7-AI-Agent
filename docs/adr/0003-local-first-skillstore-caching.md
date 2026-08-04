# ADR-0003: Local-First SkillStore Caching with Cloud AI Fallback

## Status
**Accepted**

## Context
Relying 100% on Cloud AI (Gemini API) introduces API quota dependency, WAN latency ($280\text{ms}+$), cost overhead, and vulnerability during network outages.

## Decision
Implement a **Local-First SkillStore** pattern. When an anomaly occurs, the agent first queries the local SQLite database for matching skills with confidence score $\ge 0.70$. Cloud AI is invoked ONLY on cache misses, and successful AI actions are stored back to SQLite with Exponential Weighted Moving Average ($\alpha = 0.3$) confidence score updates.

## Consequences
- **Positive:** Over **90% reduction** in Cloud AI API costs, zero-latency execution ($0.4\text{ms}$) for recurring anomalies, and offline operation capability.
- **Negative:** Cold start requires initial cloud AI requests to populate the local skill store.
