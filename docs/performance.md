# Performance Regression Testing Guidelines ⚡

System: **Beryl 7 AI Agent**  
Document Version: **v1.0.0**

---

## 1. Automated Performance Regression Workflow

To prevent performance degradation across release commits, every pull request candidate is benchmarked against the baseline commit:

```bash
# Step 1: Run baseline benchmark
git checkout main
go test -bench=. ./go-agent/... > baseline.txt

# Step 2: Run candidate benchmark
git checkout candidate-branch
go test -bench=. ./go-agent/... > candidate.txt

# Step 3: Compare results using benchstat
benchstat baseline.txt candidate.txt
```

---

## 2. Acceptance Criteria
- Memory allocation per operation must not increase by > 5%.
- Latency P99 must not regress by > 10%.
