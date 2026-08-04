# Cloud AI Cost Analysis & ROI Specification 💰

---

## 1. Gemini API Pricing Model (Gemini 2.5 Flash)

- **Input Token Rate:** $0.075 per 1,000,000 tokens
- **Output Token Rate:** $0.30 per 1,000,000 tokens
- **Average Prompt Tokens per Anomaly:** ~850 tokens
- **Average Response Tokens per Action:** ~150 tokens
- **Cost per Single AI Log Classification:** **~$0.0001088 USD** (~0.01 cents)

---

## 2. Local Skill Caching Cost Savings

Without local caching:
- 50 network anomalies per day $\times$ $0.0001088 USD = **$0.00544 USD / day** ($1.98 USD / year per router).

With **Local-First SkillStore Caching ($\ge 0.70$ confidence)**:
- 90%+ of recurring anomalies are resolved locally via SQLite ($0.00 cost).
- Only ~5 novel anomalies per day trigger Gemini API = **$0.000544 USD / day** ($0.19 USD / year per router).
- **Net Cost Savings:** **> 90% Cost Reduction**.

---

## 3. Daily Budget & Safety Controls

- **Hard Budget Limit:** Configured via `BERYL7_DAILY_BUDGET_USD=1.00` (Default: $1.00 USD / day).
- **Circuit Breaker Protection:** Stops Cloud AI requests after 5 consecutive failures for 5 minutes.
